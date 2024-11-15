package common

func SliceToMap[T any, K comparable, V any](slice []T, mapper func(i int, t T) (K, V)) map[K]V {
	m := make(map[K]V, len(slice))
	for i, t := range slice {
		k, v := mapper(i, t)
		m[k] = v
	}
	return m
}

func TransformMap[K1, K2 comparable, V1 any, V2 any](m map[K1]V1, transformer func(k K1, v V1) (K2, V2)) map[K2]V2 {
	tm := make(map[K2]V2, len(m))
	for k1, v1 := range m {
		k2, v2 := transformer(k1, v1)
		tm[k2] = v2
	}
	return tm
}

func ReverseMap[K, V comparable](input map[K]V) map[V]K {
	ret := make(map[V]K, len(input))
	for k, v := range input {
		ret[v] = k
	}
	return ret
}
