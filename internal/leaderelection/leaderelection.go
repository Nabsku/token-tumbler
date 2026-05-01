package leaderelection

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type Runner struct {
	config Config
	logger *zap.Logger
}

func NewRunner(config Config, logger *zap.Logger) *Runner {
	return &Runner{config: config, logger: logger}
}

func (r *Runner) Run(ctx context.Context, onStartedLeading func(context.Context)) error {
	client, err := inClusterClient()
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

	leaderConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   r.config.LeaseDuration,
		RenewDeadline:   r.config.RenewDeadline,
		RetryPeriod:     r.config.RetryPeriod,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStartedLeading,
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

	leaderElector, err := leaderelection.NewLeaderElector(leaderConfig)
	if err != nil {
		return fmt.Errorf("creating leader elector: %w", err)
	}

	r.logger.Info(
		"starting kubernetes leader election",
		zap.String("namespace", r.config.Namespace),
		zap.String("lease", r.config.LeaseName),
		zap.String("identity", r.config.Identity),
	)
	leaderElector.Run(ctx)
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
