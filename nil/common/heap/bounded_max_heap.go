package heap

import (
	"container/heap"
	"slices"

	"github.com/NilFoundation/nil/nil/common/check"
)

type heapAdapter[T any] struct {
	elements   []T
	comparator func(a, b T) int
}

// Len (heap.Interface) returns the number of elements in the heap.
func (h *heapAdapter[T]) Len() int {
	return len(h.elements)
}

// Less (heap.Interface) is used to determine element order.
func (h *heapAdapter[T]) Less(i, j int) bool {
	return h.comparator(h.elements[i], h.elements[j]) > 0
}

// Swap (heap.Interface) swaps elements at indices i and j.
func (h *heapAdapter[T]) Swap(i, j int) {
	h.elements[i], h.elements[j] = h.elements[j], h.elements[i]
}

// Push (heap.Interface) adds an element to the heap.
func (h *heapAdapter[T]) Push(x any) {
	t, ok := x.(T)
	check.PanicIfNot(ok)
	h.elements = append(h.elements, t)
}

// Pop (heap.Interface) removes and returns element at [Len() - 1].
func (h *heapAdapter[T]) Pop() any {
	old := h.elements
	n := len(old)
	if n == 0 {
		panic("pop from empty heap")
	}

	item := old[n-1]
	h.elements = old[:n-1]
	return item
}

// BoundedMaxHeap represents a data structure with limited capacity that stores the top elements according to comparator
type BoundedMaxHeap[T any] struct {
	capacity int
	adapter  heapAdapter[T]
}

// NewBoundedMaxHeap creates a new BoundedMaxHeap with the given capacity and comparator
func NewBoundedMaxHeap[T any](capacity int, comparator func(a, b T) int) *BoundedMaxHeap[T] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	return &BoundedMaxHeap[T]{
		capacity: capacity,
		adapter:  heapAdapter[T]{elements: make([]T, 0, capacity), comparator: comparator},
	}
}

// Add inserts an element into the heap. If the heap exceeds its capacity,
// it removes the largest element.
func (h *BoundedMaxHeap[T]) Add(element T) {
	heap.Push(&h.adapter, element)

	if h.adapter.Len() > h.capacity {
		heap.Pop(&h.adapter)
	}
}

// PopAllSorted removes all elements from the heap and returns them in sorted order (according to comparator).
func (h *BoundedMaxHeap[T]) PopAllSorted() []T {
	var result []T
	for h.adapter.Len() > 0 {
		t, ok := heap.Pop(&h.adapter).(T)
		check.PanicIfNot(ok)
		result = append(result, t)
	}
	slices.Reverse(result)
	return result
}
