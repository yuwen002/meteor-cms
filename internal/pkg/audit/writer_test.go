package audit

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriterProcessesAllJobsBeforeShutdown(t *testing.T) {
	w := NewWriter(2, 16, time.Second)
	var count atomic.Int32
	for i := 0; i < 10; i++ {
		if !w.Submit("test", func(context.Context) error {
			count.Add(1)
			return nil
		}) {
			t.Fatal("任务应被接收")
		}
	}
	w.Shutdown(2 * time.Second)
	if got := count.Load(); got != 10 {
		t.Fatalf("Shutdown 前应处理完队列内全部任务，期望 10，实际 %d", got)
	}
}

func TestWriterDropsWhenQueueFull(t *testing.T) {
	w := NewWriter(1, 1, time.Second)
	started := make(chan struct{})
	block := make(chan struct{})
	if !w.Submit("block", func(context.Context) error {
		close(started)
		<-block
		return nil
	}) {
		t.Fatal("首个任务应被接收")
	}
	<-started
	if !w.Submit("fill", func(context.Context) error { return nil }) {
		t.Fatal("队列还有一个空位，应被接收")
	}
	if w.Submit("drop", func(context.Context) error { return nil }) {
		t.Fatal("队列已满时应丢弃而不是阻塞业务请求")
	}
	close(block)
	w.Shutdown(2 * time.Second)
}

func TestWriterRecoversPanic(t *testing.T) {
	w := NewWriter(1, 4, time.Second)
	var after atomic.Bool
	w.Submit("panic", func(context.Context) error { panic("boom") })
	w.Submit("after", func(context.Context) error {
		after.Store(true)
		return nil
	})
	w.Shutdown(2 * time.Second)
	if !after.Load() {
		t.Fatal("单个任务 panic 不应带崩 worker，后续任务仍应执行")
	}
}

func TestWriterRejectsAfterShutdown(t *testing.T) {
	w := NewWriter(1, 4, time.Second)
	w.Shutdown(time.Second)
	if w.Submit("late", func(context.Context) error { return nil }) {
		t.Fatal("关闭后不应再接收任务")
	}
	w.Shutdown(time.Second)
}
