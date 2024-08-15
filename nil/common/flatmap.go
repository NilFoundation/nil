package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

type Pair[K any, V any] struct {
	Key   K
	Value V
}

// Implementation MUST be stateless.
type Comparator[V any] interface {
	Compare(V, V) int
}

type FlatMap[K any, V any, C Comparator[K]] struct {
	Items []Pair[K, V]

	comparatorStatelessnessChecked bool
}

func NewFlatMap[K comparable, V any, C Comparator[K]](initialValues map[K]V) FlatMap[K, V, C] {
	fm := FlatMap[K, V, C]{}
	if initialValues != nil {
		initializeFlatMapFromMap(&fm, initialValues)
	}
	return fm
}

func initializeFlatMapFromMap[K comparable, V any, C Comparator[K]](fm *FlatMap[K, V, C], initialValues map[K]V) {
	fm.Items = make([]Pair[K, V], 0, len(initialValues))
	for key, value := range initialValues {
		fm.Items = append(fm.Items, Pair[K, V]{Key: key, Value: value})
	}
	sort.Slice(fm.Items, func(i, j int) bool {
		return fm.compare(fm.Items[i].Key, fm.Items[j].Key) < 0
	})
}

func (fm *FlatMap[K, V, C]) ensureComparatorStatelessness(t C) {
	if !fm.comparatorStatelessnessChecked {
		fm.comparatorStatelessnessChecked = true
		v := reflect.ValueOf(t)

		if v.Kind() != reflect.Struct || v.NumField() != 0 {
			panic("Comparator must be stateless")
		}
	}
}

func (fm *FlatMap[K, V, C]) compare(a, b K) int {
	var cmp C
	// There is no way to check this statically in Go, so let's at least check it at runtime.
	fm.ensureComparatorStatelessness(cmp)
	return cmp.Compare(a, b)
}

func (fm *FlatMap[K, V, C]) Set(key K, value V) {
	idx := sort.Search(len(fm.Items), func(i int) bool {
		return fm.compare(fm.Items[i].Key, key) >= 0
	})
	if idx < len(fm.Items) && fm.compare(fm.Items[idx].Key, key) == 0 {
		fm.Items[idx].Value = value
	} else {
		fm.Items = append(fm.Items, Pair[K, V]{})
		copy(fm.Items[idx+1:], fm.Items[idx:])
		fm.Items[idx] = Pair[K, V]{Key: key, Value: value}
	}
}

func (fm *FlatMap[K, V, C]) Get(key K) (V, bool) {
	idx := sort.Search(len(fm.Items), func(i int) bool {
		return fm.compare(fm.Items[i].Key, key) >= 0
	})
	if idx < len(fm.Items) && fm.compare(fm.Items[idx].Key, key) == 0 {
		return fm.Items[idx].Value, true
	}
	var zeroValue V
	return zeroValue, false
}

func (fm *FlatMap[K, V, C]) GetOr(key K, defaultValue V) V {
	if v, ok := fm.Get(key); ok {
		return v
	}
	return defaultValue
}

func (fm *FlatMap[K, V, C]) GetOrDefault(key K) V {
	var defaultValue V
	return fm.GetOr(key, defaultValue)
}

func (fm *FlatMap[K, V, C]) Delete(key K) {
	idx := sort.Search(len(fm.Items), func(i int) bool {
		return fm.compare(fm.Items[i].Key, key) >= 0
	})
	if idx < len(fm.Items) && fm.compare(fm.Items[idx].Key, key) == 0 {
		fm.Items = append(fm.Items[:idx], fm.Items[idx+1:]...)
	}
}

func (fm FlatMap[K, V, C]) String() string {
	var builder strings.Builder
	builder.WriteString("{")
	for i, pair := range fm.Items {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("%v: %v", pair.Key, pair.Value))
	}
	builder.WriteString("}")
	return builder.String()
}

func (fm FlatMap[K, V, C]) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, buf, 4096)
	stream.WriteObjectStart()
	for i, pair := range fm.Items {
		if i > 0 {
			stream.WriteMore()
		}
		k, err := json.Marshal(pair.Key)
		if err != nil {
			return nil, err
		}
		stream.WriteObjectField(string(k))
		stream.WriteVal(pair.Value)
	}
	stream.WriteObjectEnd()
	err := stream.Flush()
	return buf.Bytes(), err
}

func (fm *FlatMap[K, V, C]) UnmarshalJSON(data []byte) error {
	var m map[string]V
	if err := jsoniter.Unmarshal(data, &m); err != nil {
		return err
	}
	fm.Items = make([]Pair[K, V], 0, len(m))
	for key, value := range m {
		var k K
		if err := json.Unmarshal([]byte(key), &k); err != nil {
			return err
		}
		fm.Items = append(fm.Items, Pair[K, V]{Key: k, Value: value})
	}
	sort.Slice(fm.Items, func(i, j int) bool {
		return fm.compare(fm.Items[i].Key, fm.Items[j].Key) < 0
	})
	return nil
}
