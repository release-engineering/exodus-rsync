package syncutil

import "sync"

func RunWithGroup(count int, fn func(), close func()) {
	wg := sync.WaitGroup{}
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			fn()
			wg.Done()
		}()
	}

	wg.Wait()
	close()
}
