package leaderelection

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const activeLeaderWorkDrainTimeout = 5 * time.Second

type leaderElectorRunner interface {
	Run(context.Context)
}

var (
	inClusterClientFunc  = inClusterClient
	newLeaderElectorFunc = func(config leaderelection.LeaderElectionConfig) (leaderElectorRunner, error) {
		return leaderelection.NewLeaderElector(config)
	}
)

type Runner struct {
	config Config
	logger *zap.Logger
}

func NewRunner(config Config, logger *zap.Logger) *Runner {
	return &Runner{config: config, logger: logger}
}

func (r *Runner) Run(ctx context.Context, onStartedLeading func(context.Context)) error {
	client, err := inClusterClientFunc()
	if err != nil {
		return err
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      r.config.LeaseName,
			Namespace: r.config.Namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: r.config.Identity,
		},
	}

	activeLeadership := newActiveLeadership()

	leaderConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   r.config.LeaseDuration,
		RenewDeadline:   r.config.RenewDeadline,
		RetryPeriod:     r.config.RetryPeriod,
		ReleaseOnCancel: false,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				activeLeadership.run(leaderCtx, onStartedLeading)
			},
			OnStoppedLeading: func() {
				r.logger.Warn("leader election lost; stopping active runner", zap.String("lease", r.config.LeaseName))
			},
			OnNewLeader: func(identity string) {
				if identity == r.config.Identity {
					r.logger.Info("acquired leader election lease", zap.String("lease", r.config.LeaseName))
					return
				}
				r.logger.Info("observed leader election holder", zap.String("lease", r.config.LeaseName), zap.String("leader", identity))
			},
		},
	}

	leaderElector, err := newLeaderElectorFunc(leaderConfig)
	if err != nil {
		return fmt.Errorf("creating leader elector: %w", err)
	}

	r.logger.Info(
		"starting kubernetes leader election",
		zap.String("namespace", r.config.Namespace),
		zap.String("lease", r.config.LeaseName),
		zap.String("identity", r.config.Identity),
	)

	electorCtx, cancelElector := context.WithCancel(context.Background())
	defer cancelElector()

	electorDone := make(chan struct{})
	go func() {
		defer close(electorDone)
		leaderElector.Run(electorCtx)
	}()

	select {
	case <-ctx.Done():
		if ok := activeLeadership.stopAndWait(activeLeaderWorkDrainTimeout); !ok {
			r.logger.Warn(
				"timed out waiting for leader work to stop before terminating election",
				zap.Duration("timeout", activeLeaderWorkDrainTimeout),
				zap.String("lease", r.config.LeaseName),
			)
		}
		cancelElector()
		<-electorDone
	case <-electorDone:
	}

	return nil
}

func inClusterClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("loading in-cluster kubernetes config for leader election: %w", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client for leader election: %w", err)
	}
	return client, nil
}

type activeLeadership struct {
	mu           sync.Mutex
	shuttingDown bool
	activeDone   chan struct{}
	activeCancel context.CancelFunc
}

func newActiveLeadership() *activeLeadership {
	return &activeLeadership{}
}

func (a *activeLeadership) run(leaderCtx context.Context, callback func(context.Context)) {
	workCtx, stopForward, done, ok := a.start()
	if !ok {
		return
	}
	defer func() {
		close(stopForward)
		a.finish(done)
	}()

	go func() {
		select {
		case <-leaderCtx.Done():
			a.cancelActive()
		case <-stopForward:
		}
	}()

	callback(workCtx)
}

func (a *activeLeadership) start() (context.Context, chan struct{}, chan struct{}, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.shuttingDown {
		return nil, nil, nil, false
	}

	workCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.activeCancel = cancel
	a.activeDone = done

	return workCtx, make(chan struct{}), done, true
}

func (a *activeLeadership) finish(done chan struct{}) {
	a.mu.Lock()
	if a.activeDone == done {
		a.activeCancel = nil
		a.activeDone = nil
	}
	a.mu.Unlock()

	close(done)
}

func (a *activeLeadership) cancelActive() {
	a.mu.Lock()
	cancel := a.activeCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *activeLeadership) stopAndWait(timeout time.Duration) bool {
	a.mu.Lock()
	a.shuttingDown = true
	cancel := a.activeCancel
	done := a.activeDone
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done == nil {
		return true
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
