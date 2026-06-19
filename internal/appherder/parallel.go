package appherder

import (
	"context"
	"sync"
)

// parallelMap applies fn to each item with at most limit concurrent calls,
// returning results in input order.
func parallelMap[T, R any](ctx context.Context, items []T, limit int, fn func(context.Context, T) R) []R {
	if limit < 1 {
		limit = 1
	}
	results := make([]R, len(items))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, item T) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = fn(ctx, item)
		}(i, item)
	}
	wg.Wait()
	return results
}
