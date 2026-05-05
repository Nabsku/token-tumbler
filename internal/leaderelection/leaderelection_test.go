package leaderelection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	clientleaderelection "k8s.io/client-go/tools/leaderelection"
)

type stubLeaderElector struct {
	run func(context.Context)
}

func (s stubLeaderElector) Run(ctx context.Context) {
	s.run(ctx)
}

func TestRunWaitsForLeaderWorkBeforeStoppingElector(t *testing.T) {
	originalInClusterClientFunc := inClusterClientFunc
	originalNewLeaderElectorFunc := newLeaderElectorFunc
	t.Cleanup(func() {
		inClusterClientFunc = originalInClusterClientFunc
		newLeaderElectorFunc = originalNewLeaderElectorFunc
	})

	inClusterClientFunc = func() (kubernetes.Interface, error) {
		return kubernetesfake.NewSimpleClientset(), nil
	}

	leaderCtx := context.Background()
	electorStarted := make(chan struct{})
	electorCanceled := make(chan struct{})
	callbackStarted := make(chan struct{})
	callbackCanceled := make(chan struct{})
	allowCallbackExit := make(chan struct{})
	callbackReturned := make(chan struct{})

	newLeaderElectorFunc = func(config clientleaderelection.LeaderElectionConfig) (leaderElectorRunner, error) {
		require.False(t, config.ReleaseOnCancel)

		return stubLeaderElector{
			run: func(electorCtx context.Context) {
				close(electorStarted)
				go config.Callbacks.OnStartedLeading(leaderCtx)
				<-electorCtx.Done()
				close(electorCanceled)
			},
		}, nil
	}

	runner := NewRunner(Config{
		Enabled:       true,
		Namespace:     "default",
		LeaseName:     "token-tumbler",
		Identity:      "pod-1",
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- runner.Run(ctx, func(workCtx context.Context) {
			close(callbackStarted)
			<-workCtx.Done()
			close(callbackCanceled)
			<-allowCallbackExit
			close(callbackReturned)
		})
	}()

	waitForSignal(t, electorStarted, "elector start")
	waitForSignal(t, callbackStarted, "leader callback start")

	cancel()

	waitForSignal(t, callbackCanceled, "leader callback cancellation")

	select {
	case <-electorCanceled:
		t.Fatal("elector context canceled before leader callback returned")
	case <-time.After(200 * time.Millisecond):
	}

	close(allowCallbackExit)

	waitForSignal(t, callbackReturned, "leader callback return")
	waitForSignal(t, electorCanceled, "elector cancellation")

	require.NoError(t, <-runDone)
}

func waitForSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
