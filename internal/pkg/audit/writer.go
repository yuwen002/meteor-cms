// Package audit 提供审计日志的异步写入队列。
//
// 直接 go func 写日志在 DB 抖动或流量突增时会无限创建 goroutine，进程重启还会
// 丢掉在途记录。这里用有界队列 + 固定 worker 收口，并提供优雅关闭等待。
package audit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type job struct {
	name string
	fn   func(context.Context) error
}

// Writer 是有界的异步日志写入器。队列满时丢弃任务并记 error 日志，
// 宁可少一条审计记录，也不能把业务请求拖垮或让 goroutine 无限增长。
type Writer struct {
	queue   chan job
	timeout time.Duration
	wg      sync.WaitGroup
	closed  atomic.Bool
	dropped atomic.Int64
	once    sync.Once
}

var (
	defaultOnce    sync.Once
	defaultWriter  *Writer
	defaultWorkers = 4
	defaultQueue   = 1024
)

// Default 返回进程级共享写入器（懒初始化）。
func Default() *Writer {
	defaultOnce.Do(func() {
		defaultWriter = NewWriter(defaultWorkers, defaultQueue, 3*time.Second)
	})
	return defaultWriter
}

func NewWriter(workers, queueSize int, timeout time.Duration) *Writer {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	w := &Writer{queue: make(chan job, queueSize), timeout: timeout}
	w.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go w.loop()
	}
	return w
}

func (w *Writer) loop() {
	defer w.wg.Done()
	for j := range w.queue {
		w.run(j)
	}
}

func (w *Writer) run(j job) {
	// 单条写入 panic 不能带崩整个 worker
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("审计日志 %s 写入 panic: %v", j.name, r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()
	if err := j.fn(ctx); err != nil {
		logx.Errorf("审计日志 %s 写入失败: %v", j.name, err)
	}
}

// Submit 投递一条写入任务；队列满或已关闭时返回 false（不阻塞业务请求）。
func (w *Writer) Submit(name string, fn func(context.Context) error) (queued bool) {
	if w.closed.Load() {
		w.dropped.Add(1)
		return false
	}
	// Shutdown 与 Submit 并发时 send on closed channel 会 panic，这里兜底
	defer func() {
		if recover() != nil {
			logx.Errorf("审计日志队列已关闭，丢弃 %s", name)
			w.dropped.Add(1)
			queued = false
		}
	}()
	select {
	case w.queue <- job{name: name, fn: fn}:
		return true
	default:
		w.dropped.Add(1)
		logx.Errorf("审计日志队列已满，丢弃 %s", name)
		return false
	}
}

// Stats 返回队列积压与累计丢弃数，供 /admin/monitor 观测写入压力。
func (w *Writer) Stats() (depth int, dropped int64) {
	return len(w.queue), w.dropped.Load()
}

// Shutdown 停止接收新任务，处理完队列内剩余任务后返回；超时则放弃等待。
func (w *Writer) Shutdown(wait time.Duration) {
	w.once.Do(func() {
		w.closed.Store(true)
		close(w.queue)
	})
	if wait <= 0 {
		wait = 5 * time.Second
	}
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(wait):
		logx.Errorf("等待审计日志写入超时，部分记录可能未落库")
	}
}

// Submit 使用进程级共享写入器投递任务。
func Submit(name string, fn func(context.Context) error) bool {
	return Default().Submit(name, fn)
}

// Stats 返回共享写入器的队列积压与累计丢弃数。
func Stats() (int, int64) {
	return Default().Stats()
}

// Shutdown 等待共享写入器处理完在途任务。
func Shutdown(wait time.Duration) {
	Default().Shutdown(wait)
}
