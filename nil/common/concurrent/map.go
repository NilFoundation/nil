package concurrent

import (
	iter "iter"
	sync "sync"
)

type Map[K comparable, T any] struct {
	m  map[K]T
	mu sync.RWMutex
}

func NewMap[K comparable, T any]() *Map[K, T] {
	return &Map[K, T]{
		m: make(map[K]T),
	}
}

func (m *Map[K, T]) Get(k K) (res T, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res, ok = m.m[k]
	return res, ok
}

func (m *Map[K, T]) Put(k K, v T) (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.m[k]
	m.m[k] = v
	return old, ok
}

func (m *Map[K, T]) Do(k K, fn func(T, bool) (T, bool)) (after T, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.m[k]
	nv, save := fn(val, ok)
	if save {
		m.m[k] = nv
	}
	return nv, ok
}

func (m *Map[K, T]) DoAndStore(k K, fn func(t T, ok bool) T) (after T, ok bool) {
	return m.Do(k, func(t T, b bool) (T, bool) {
		res := fn(t, b)
		return res, true
	})
}

func (m *Map[K, T]) Iterate() iter.Seq2[K, T] {
	type Yield = func(K, T) bool
	return func(yield Yield) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for k, v := range m.m {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (m *Map[K, T]) Delete(k K) (t T, deleted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.m[k]
	if !ok {
		return t, false
	}
	delete(m.m, k)
	return val, true
}
