package serialization

import (
	"bytes"
	"slices"
)

type KeyValue struct {
	Key   []byte
	Value []byte
}

// MapHolder is a wrapper around a map[string][]byte that can be serialized.
type MapHolder struct {
	// Data is a sorted list of key-value pairs.
	Data []KeyValue
}

func NewMapHolder(m map[string][]byte) *MapHolder {
	keyValues := make([]KeyValue, 0, len(m))
	for key, value := range m {
		keyValues = append(keyValues, KeyValue{[]byte(key), value})
	}
	slices.SortFunc(keyValues, func(a, b KeyValue) int {
		return bytes.Compare(a.Key, b.Key)
	})
	return &MapHolder{Data: keyValues}
}

func (m *MapHolder) ToMap() map[string][]byte {
	result := make(map[string][]byte, len(m.Data))
	for _, kv := range m.Data {
		result[string(kv.Key)] = kv.Value
	}
	return result
}
