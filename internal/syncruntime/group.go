package syncruntime

import (
	"context"
	"sync"
)

type WorkerGroup struct {
	Workers []Worker
}

func (g WorkerGroup) Run(ctx context.Context) error {
	if len(g.Workers) == 0 {
		return nil
	}

	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, len(g.Workers))
	var wg sync.WaitGroup
	for _, worker := range g.Workers {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- worker.Run(groupCtx)
		}()
	}

	firstErr := <-errs
	cancel()
	wg.Wait()
	return firstErr
}
