package dispatch

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestFanOutKeepsSuccessfulResultsInInputOrder(t *testing.T) {
	var started sync.WaitGroup
	started.Add(3)
	results := FanOut(context.Background(), []int{1, 2, 3}, func(_ context.Context, item int) (string, error) {
		started.Done()
		started.Wait()
		switch item {
		case 2:
			return "", errors.New("unavailable")
		case 3:
			panic("query failure")
		default:
			return "one", nil
		}
	})
	want := []FanOutResult[int, string]{{Item: 1, Value: "one"}}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("FanOut() = %#v, want %#v", results, want)
	}
}

func TestFanOutSupportsNonComparableItems(t *testing.T) {
	items := [][]string{{"first"}, {"second"}}
	results := FanOut(context.Background(), items, func(_ context.Context, item []string) (int, error) {
		return len(item[0]), nil
	})
	if len(results) != 2 || results[0].Value != 5 || results[1].Value != 6 {
		t.Fatalf("FanOut() = %#v, want ordered non-comparable results", results)
	}
}

func TestFanOutCancelledContextReturnsWithoutWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	results := make(chan []FanOutResult[int, int], 1)
	go func() {
		results <- FanOut(ctx, []int{1}, func(ctx context.Context, _ int) (int, error) {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		})
	}()
	<-started
	cancel()
	if got := <-results; got != nil {
		t.Fatalf("FanOut() = %#v, want nil after cancellation", got)
	}
}
