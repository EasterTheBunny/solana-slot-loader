package async

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	ErrProcessStopped   = fmt.Errorf("worker process has stopped")
	ErrContextCancelled = fmt.Errorf("worker context cancelled")
)

type Job func(context.Context)

type worker struct {
	Name  string
	Queue chan *worker
}

func (w *worker) Do(ctx context.Context, job Job) {
	if ctx.Err() == nil {
		job(ctx)
	}

	// put itself back on the queue when done
	select {
	case w.Queue <- w:
	default:
	}
}

type WorkerGroup struct {
	maxWorkers    int
	activeWorkers int
	workers       chan *worker

	queue         *Queue[Job]
	input         chan Job
	chInputNotify chan struct{}

	// channels used to stop processing
	chStopInputs     chan struct{}
	chStopProcessing chan struct{}
	queueClosed      atomic.Bool

	// service state management
	svcCtx    context.Context
	svcCancel context.CancelFunc
	once      sync.Once
}

func NewWorkerGroup(workers int) *WorkerGroup {
	svcCtx, svcCancel := context.WithCancel(context.Background())
	wg := &WorkerGroup{
		maxWorkers:       workers,
		workers:          make(chan *worker, workers),
		queue:            &Queue[Job]{},
		input:            make(chan Job, 1),
		chInputNotify:    make(chan struct{}, 1),
		chStopInputs:     make(chan struct{}),
		chStopProcessing: make(chan struct{}),
		svcCtx:           svcCtx,
		svcCancel:        svcCancel,
	}

	go wg.run()

	return wg
}

// Do adds a new work item onto the work queue. This function blocks until
// the work queue clears up or the context is cancelled.
func (wg *WorkerGroup) Do(ctx context.Context, job Job) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w; work not added to queue", ErrContextCancelled)
	}

	if wg.queueClosed.Load() {
		return fmt.Errorf("%w; work not added to queue", ErrProcessStopped)
	}

	select {
	case wg.input <- job:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w; work not added to queue", ErrContextCancelled)
	case <-wg.svcCtx.Done():
		return fmt.Errorf("%w; work not added to queue", ErrProcessStopped)
	}
}

func (wg *WorkerGroup) Stop() {
	wg.once.Do(func() {
		wg.svcCancel()
		wg.queueClosed.Store(true)
		wg.chStopInputs <- struct{}{}
	})
}

func (wg *WorkerGroup) processQueue() {
	for {
		if wg.queue.Len() == 0 {
			break
		}

		value, err := wg.queue.Pop()

		// an error from pop means there is nothing to pop
		// the length check above should protect from that, but just in case
		// this error also breaks the loop
		if err != nil {
			break
		}

		wg.doJob(value)
	}
}

func (wg *WorkerGroup) runQueuing() {
	for {
		select {
		case item := <-wg.input:
			wg.queue.Add(item)

			// notify that new work item came in
			// drop if notification channel is full
			select {
			case wg.chInputNotify <- struct{}{}:
			default:
			}
		case <-wg.chStopInputs:
			wg.chStopProcessing <- struct{}{}
			return
		}
	}
}

func (wg *WorkerGroup) runProcessing() {
	for {
		select {
		// watch notification channel and begin processing queue
		// when notification occurs
		case <-wg.chInputNotify:
			wg.processQueue()
		case <-wg.chStopProcessing:
			return
		}
	}
}

func (wg *WorkerGroup) run() {
	// start listening on the input channel for new jobs
	go wg.runQueuing()

	// main run loop for queued jobs
	wg.runProcessing()

	// run the job queue one more time just in case some
	// new work items snuck in
	wg.processQueue()
}

func (wg *WorkerGroup) doJob(job Job) {
	var wkr *worker

	// no read or write locks on activeWorkers or maxWorkers because it's
	// assumed the job loop is a single process reading from the job queue
	if wg.activeWorkers < wg.maxWorkers {
		// create a new worker
		wkr = &worker{
			Name:  fmt.Sprintf("worker-%d", wg.activeWorkers+1),
			Queue: wg.workers,
		}
		wg.activeWorkers++
	} else {
		// wait for a worker to be available
		wkr = <-wg.workers
	}

	// have worker do the work
	go wkr.Do(wg.svcCtx, job)
}

type Queue[T any] struct {
	mu     sync.RWMutex
	values []T
}

func (q *Queue[T]) Add(values ...T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.values = append(q.values, values...)
}

func (q *Queue[T]) Pop() (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.values) == 0 {
		return getZero[T](), fmt.Errorf("no values to return")
	}

	val := q.values[0]

	if len(q.values) > 1 {
		q.values = q.values[1:]
	} else {
		q.values = []T{}
	}

	return val, nil
}

func (q *Queue[T]) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.values)
}

func getZero[T any]() T {
	var result T
	return result
}
