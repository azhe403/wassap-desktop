//go:build windows

package webview2

import "sync"

// callback is one unit of work queued for the session's UI thread.
type callback struct {
	fn     func() error
	result chan error
}

// callbackQueue is a FIFO queue of pending callbacks. It replaces the old
// single-slot atomic.Pointer[callback], which lost a pending call whenever
// two callers raced (the overwritten caller blocked for the full timeout and
// its slot could later be released out from under a newer callback).
type callbackQueue struct {
	mu   sync.Mutex
	list []*callback
}

func (q *callbackQueue) push(c *callback) {
	q.mu.Lock()
	q.list = append(q.list, c)
	q.mu.Unlock()
}

// pop returns the oldest pending callback, or nil when empty.
func (q *callbackQueue) pop() *callback {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.list) == 0 {
		return nil
	}
	c := q.list[0]
	q.list = q.list[1:]
	return c
}

// remove drops c from the queue if it is still pending (used on timeout or
// when the message could not be posted).
func (q *callbackQueue) remove(c *callback) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, v := range q.list {
		if v == c {
			q.list = append(q.list[:i], q.list[i+1:]...)
			return
		}
	}
}

func (q *callbackQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.list)
}
