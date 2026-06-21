// Package scheduler запускає фонові задачі з фіксованим інтервалом.
//
// Всі задачі отримують context і зупиняються при його скасуванні —
// graceful shutdown main.go автоматично зупиняє і scheduler.
//
// Щоб додати нову задачу достатньо одного виклику Register в main.go,
// без змін в самому scheduler.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/okochadmytro/tradetracker/internal/metrics"
)

// Job описує одну фонову задачу.
type Job struct {
	// Name використовується лише для логів
	Name     string
	Interval time.Duration
	// Fn викликається кожен Interval. Якщо виконання довше за Interval —
	// наступний тік пропускається (ticker не акумулює ticks).
	Fn func(ctx context.Context)
}

// Scheduler керує набором фонових задач.
type Scheduler struct {
	jobs   []Job
	logger *slog.Logger
}

func New(logger *slog.Logger) *Scheduler {
	return &Scheduler{logger: logger}
}

// Register додає задачі до scheduler.
// Викликати до Start.
func (s *Scheduler) Register(jobs ...Job) {
	s.jobs = append(s.jobs, jobs...)
}

// Start запускає всі зареєстровані задачі в окремих goroutines.
// Повертає одразу. Зупиняється коли ctx скасовано.
func (s *Scheduler) Start(ctx context.Context) {
	for _, job := range s.jobs {
		go s.run(ctx, job)
	}
	s.logger.Info("scheduler: started", "jobs", len(s.jobs))
}

func (s *Scheduler) run(ctx context.Context, job Job) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	s.logger.Info("scheduler: job registered", "job", job.Name, "interval", job.Interval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler: job stopped", "job", job.Name)
			return
		case <-ticker.C:
			s.logger.Debug("scheduler: running job", "job", job.Name)
			t := time.Now()
			job.Fn(ctx)
			metrics.SchedulerJobDuration.WithLabelValues(job.Name).Observe(time.Since(t).Seconds())
		}
	}
}
