package common

import (
	"iter"

	"golang.org/x/exp/constraints"
)

// Adapters based on those proposed for inclusion in [x/exp/xiter](https://github.com/golang/go/issues/61898),
// while it is still not available to us.
// Added as needed.

func Filter[V any](seq iter.Seq[V], filter func(V) bool) iter.Seq[V] {
	return func(yield func(V) bool) {
		for v := range seq {
			if filter(v) && !yield(v) {
				return
			}
		}
	}
}

func Transform[In, Out any](seq iter.Seq[In], transformer func(In) Out) iter.Seq[Out] {
	return func(yield func(Out) bool) {
		for in := range seq {
			if !yield(transformer(in)) {
				return
			}
		}
	}
}

func Range[T constraints.Integer](start, end T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := start; i < end; i++ {
			if !yield(i) {
				return
			}
		}
	}
}
