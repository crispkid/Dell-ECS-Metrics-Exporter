package collector

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"dell-ecs-metrics-exporter/internal/config"
)

type Scheduler struct {
	runners []*Runner
	config  config.CollectorConfig
	wg      sync.WaitGroup
}

func NewScheduler(runners []*Runner, settings config.CollectorConfig) *Scheduler {
	return &Scheduler{runners: runners, config: settings}
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, runner := range s.runners {
		intervals := runner.config.Intervals.Resolve(s.config.Intervals)
		tasks := []struct {
			name     string
			interval time.Duration
			run      func(context.Context) error
		}{
			{"cluster", intervals["cluster"], runner.CollectCluster},
			{"node", intervals["node"], runner.CollectNodes},
			{"node-resources", intervals["node"], runner.CollectNodeResources},
			{"performance", intervals["performance"], runner.CollectPerformance},
			{"namespace", intervals["namespace"], runner.CollectNamespaces},
			{"bucket", intervals["bucket"], runner.CollectBuckets},
			{"replication", intervals["replication"], runner.CollectReplication},
			{"recovery", intervals["recovery"], runner.CollectRecovery},
		}
		for _, task := range tasks {
			task := task
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				runScheduled(ctx, runner, task.name, task.interval, s.config.JitterRatio, task.run)
			}()
		}
	}
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func runScheduled(
	ctx context.Context,
	runner *Runner,
	name string,
	interval time.Duration,
	jitterRatio float64,
	function func(context.Context) error,
) {
	if initial := jitterDelay(interval, jitterRatio); initial > 0 {
		if !waitForSchedule(ctx, initial) {
			return
		}
	}
	runner.Run(ctx, name, function)
	for {
		delay := interval
		delay += jitterDelay(interval, jitterRatio)
		if !waitForSchedule(ctx, delay) {
			return
		}
		runner.Run(ctx, name, function)
	}
}

func jitterDelay(interval time.Duration, jitterRatio float64) time.Duration {
	if jitterRatio <= 0 {
		return 0
	}
	window := int64(float64(interval) * jitterRatio)
	if window <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(window) + 1)
}

func waitForSchedule(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
