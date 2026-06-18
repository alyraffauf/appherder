package appherder

import (
	"context"
	"reflect"
	"testing"
)

func TestParallelMapPreservesInputOrder(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	got := parallelMap(context.Background(), items, 3, func(_ context.Context, n int) int {
		return n * n
	})
	want := []int{1, 4, 9, 16, 25, 36, 49}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParallelMapEmpty(t *testing.T) {
	got := parallelMap(context.Background(), []int{}, 4, func(_ context.Context, n int) int { return n })
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}
