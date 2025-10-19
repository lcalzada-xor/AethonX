// internal/platform/workerpool/worker_pool.go
package workerpool

import (
	"context"
	"sync"
	"time"

	"aethonx/internal/platform/logx"
)

// Task representa una tarea a ejecutar en el worker pool.
type Task interface {
	// Execute ejecuta la tarea
	Execute(ctx context.Context) error

	// Priority retorna la prioridad de la tarea (mayor = más prioritario)
	Priority() int

	// Weight retorna el peso/costo estimado de la tarea (0-100)
	Weight() int

	// Name retorna el nombre de la tarea
	Name() string
}

// Scheduler define la estrategia de scheduling.
type Scheduler interface {
	// Schedule ordena las tareas según la estrategia
	Schedule(tasks []Task) []Task

	// Name retorna el nombre del scheduler
	Name() string
}

// WorkerPool gestiona la ejecución concurrente de tareas con scheduling.
type WorkerPool struct {
	workers   int
	scheduler Scheduler
	logger    logx.Logger

	// Channels
	taskQueue chan Task
	results   chan TaskResult

	// Control
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// TaskResult representa el resultado de una tarea.
type TaskResult struct {
	Task     Task
	Error    error
	Duration time.Duration
}

// WorkerPoolConfig configura el worker pool.
type WorkerPoolConfig struct {
	Workers   int
	Scheduler Scheduler
	Logger    logx.Logger
}

// NewWorkerPool crea un nuevo worker pool.
func NewWorkerPool(cfg WorkerPoolConfig) *WorkerPool {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.Scheduler == nil {
		cfg.Scheduler = NewPriorityScheduler()
	}
	if cfg.Logger == nil {
		cfg.Logger = logx.New()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workers:   cfg.Workers,
		scheduler: cfg.Scheduler,
		logger:    cfg.Logger.With("component", "worker-pool"),
		taskQueue: make(chan Task, cfg.Workers*2), // Buffer 2x workers
		results:   make(chan TaskResult, cfg.Workers*2),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start inicia el worker pool.
func (wp *WorkerPool) Start() {
	wp.logger.Info("starting worker pool", "workers", wp.workers, "scheduler", wp.scheduler.Name())

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker es el goroutine que procesa tareas.
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	wp.logger.Debug("worker started", "worker_id", id)

	for {
		select {
		case <-wp.ctx.Done():
			wp.logger.Debug("worker stopped", "worker_id", id)
			return

		case task, ok := <-wp.taskQueue:
			if !ok {
				wp.logger.Debug("task queue closed, worker stopping", "worker_id", id)
				return
			}

			wp.executeTask(id, task)
		}
	}
}

// executeTask ejecuta una tarea individual.
func (wp *WorkerPool) executeTask(workerID int, task Task) {
	start := time.Now()

	wp.logger.Debug("executing task",
		"worker_id", workerID,
		"task", task.Name(),
		"priority", task.Priority(),
		"weight", task.Weight(),
	)

	err := task.Execute(wp.ctx)
	duration := time.Since(start)

	wp.logger.Debug("task completed",
		"worker_id", workerID,
		"task", task.Name(),
		"duration_ms", duration.Milliseconds(),
		"error", err != nil,
	)

	// Enviar resultado
	select {
	case wp.results <- TaskResult{
		Task:     task,
		Error:    err,
		Duration: duration,
	}:
	case <-wp.ctx.Done():
		// Pool stopped, discard result
	}
}

// Submit envía tareas al pool con scheduling.
func (wp *WorkerPool) Submit(tasks []Task) []TaskResult {
	if len(tasks) == 0 {
		return []TaskResult{}
	}

	// Schedule tasks
	scheduledTasks := wp.scheduler.Schedule(tasks)

	wp.logger.Info("submitting tasks",
		"total", len(scheduledTasks),
		"scheduler", wp.scheduler.Name(),
	)

	// Enviar tareas al queue
	go func() {
		for _, task := range scheduledTasks {
			select {
			case wp.taskQueue <- task:
			case <-wp.ctx.Done():
				return
			}
		}
	}()

	// Recolectar resultados
	results := make([]TaskResult, 0, len(tasks))
	for i := 0; i < len(tasks); i++ {
		select {
		case result := <-wp.results:
			results = append(results, result)
		case <-wp.ctx.Done():
			wp.logger.Warn("pool stopped while waiting for results")
			return results
		}
	}

	return results
}

// Stop detiene el worker pool.
func (wp *WorkerPool) Stop() {
	wp.logger.Info("stopping worker pool")

	// Cancel context to signal workers
	wp.cancel()

	// Close task queue
	close(wp.taskQueue)

	// Wait for all workers to finish
	wp.wg.Wait()

	// Close results channel
	close(wp.results)

	wp.logger.Info("worker pool stopped")
}

// Stats retorna estadísticas del worker pool.
func (wp *WorkerPool) Stats() WorkerPoolStats {
	return WorkerPoolStats{
		Workers:      wp.workers,
		SchedulerName: wp.scheduler.Name(),
		QueueSize:    len(wp.taskQueue),
		ResultsSize:  len(wp.results),
	}
}

// WorkerPoolStats contiene estadísticas del worker pool.
type WorkerPoolStats struct {
	Workers      int
	SchedulerName string
	QueueSize    int
	ResultsSize  int
}
