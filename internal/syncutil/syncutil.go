package syncutil

import "sync"

// RunWithGroup is a helper to implement the "Bounded parallelism" pattern
// described at https://blog.golang.org/pipelines.
//
// It will spawn n goroutines all running fn, wait for them to complete,
// then run close.
//
// Generally, fn would be a function sending values to some channel,
// and close would be a function to close that channel.
func RunWithGroup(n int, fn func(), close func()) {
	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			fn()
			wg.Done()
		}()
	}

	wg.Wait()
	close()
}
