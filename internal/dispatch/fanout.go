package dispatch

import (
	"context"
	"errors"
	"fmt"
)

// FanOutResult pairs a successful query value with its input item.
type FanOutResult[Item, Value any] struct {
	Item  Item
	Value Value
}

type fanOutResult[Item, Value any] struct {
	FanOutResult[Item, Value]
	index int
	err   error
}

// FanOut concurrently queries every item. Failed and panicking queries are
// excluded; successful pairs retain input order.
func FanOut[Item, Value any](ctx context.Context, items []Item, query func(context.Context, Item) (Value, error)) []FanOutResult[Item, Value] {
	outcomes, completed := fanOut(ctx, items, query)
	if !completed {
		return nil
	}
	results := make([]FanOutResult[Item, Value], 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.err == nil {
			results = append(results, outcome.FanOutResult)
		}
	}
	return results
}

func fanOut[Item, Value any](ctx context.Context, items []Item, query func(context.Context, Item) (Value, error)) ([]fanOutResult[Item, Value], bool) {
	outcomes := make(chan fanOutResult[Item, Value], len(items))
	for index, item := range items {
		go func(index int, item Item) {
			outcome := fanOutResult[Item, Value]{
				FanOutResult: FanOutResult[Item, Value]{Item: item},
				index:        index,
				err:          errors.New("query did not complete"),
			}
			defer func() {
				if recovered := recover(); recovered != nil {
					outcome.err = fmt.Errorf("query panicked: %v", recovered)
				}
				outcomes <- outcome
			}()
			if query == nil {
				outcome.err = errors.New("nil query")
				return
			}
			outcome.Value, outcome.err = query(ctx, item)
		}(index, item)
	}

	results := make([]fanOutResult[Item, Value], len(items))
	for range items {
		select {
		case <-ctx.Done():
			return nil, false
		case outcome := <-outcomes:
			results[outcome.index] = outcome
		}
	}
	return results, true
}
