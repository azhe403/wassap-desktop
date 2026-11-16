//go:build windows

package webview2

import (
	"fmt"
	"sync"
	"testing"
)

func TestCallbackQueueIsFIFO(t *testing.T) {
	var q callbackQueue
	a := &callback{fn: func() error { return nil }, result: make(chan error, 1)}
	b := &callback{fn: func() error { return nil }, result: make(chan error, 1)}
	c := &callback{fn: func() error { return nil }, result: make(chan error, 1)}

	q.push(a)
	q.push(b)
	q.push(c)

	if got := q.pop(); got != a {
		t.Errorf("first pop = %p, want a", got)
	}
	if got := q.pop(); got != b {
		t.Errorf("second pop = %p, want b", got)
	}
	if got := q.pop(); got != c {
		t.Errorf("third pop = %p, want c", got)
	}
	if got := q.pop(); got != nil {
		t.Errorf("pop on empty queue = %p, want nil", got)
	}
}

func TestCallbackQueueRemoveDropsOnlyTheTarget(t *testing.T) {
	var q callbackQueue
	a := &callback{fn: func() error { return nil }, result: make(chan error, 1)}
	b := &callback{fn: func() error { return nil }, result: make(chan error, 1)}

	q.push(a)
	q.push(b)
	q.remove(a)

	if got := q.pop(); got != b {
		t.Errorf("pop after remove(a) = %p, want b", got)
	}
	if got := q.pop(); got != nil {
		t.Errorf("queue should only hold b, got %p", got)
	}
	q.remove(a) // removing an absent callback must not panic or touch others
	q.push(a)
	q.remove(a)
	if q.Len() != 0 {
		t.Errorf("queue length = %d, want 0", q.Len())
	}
}

func TestCallbackQueueConcurrentPushDrain(t *testing.T) {
	var q callbackQueue
	const producers = 50
	const each = 20

	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				q.push(&callback{fn: func() error { return fmt.Errorf("p%d-i%d", p, i) }, result: make(chan error, 1)})
			}
		}()
	}
	wg.Wait()
	if q.Len() != producers*each {
		t.Fatalf("queue length = %d, want %d", q.Len(), producers*each)
	}

	seen := make(map[error]int)
	for {
		c := q.pop()
		if c == nil {
			break
		}
		err := c.fn()
		seen[err]++
	}
	if len(seen) != producers*each {
		t.Fatalf("popped %d distinct errors, want %d (loss or duplication)", len(seen), producers*each)
	}
	for err, n := range seen {
		if n != 1 {
			t.Errorf("error %v popped %d times", err, n)
		}
	}
}
