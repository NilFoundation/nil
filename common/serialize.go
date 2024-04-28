package common

import (
	"bytes"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// less efective encoding with strict wire representation
func SerializeBinaryPersistent[T any](val T) ([]byte, error) {
	var buf bytes.Buffer
	em, _ := cbor.CoreDetEncOptions().EncMode()
	err := em.NewEncoder(&buf).Encode(val)
	return buf.Bytes(), err
}

func MustSerializeBinaryPersistent[T any](val T) []byte {
	var buf bytes.Buffer
	em, _ := cbor.CoreDetEncOptions().EncMode()
	if err := em.NewEncoder(&buf).Encode(val); err != nil {
		panic("unexpected serialization error")
	}
	return buf.Bytes()
}

func DeserializeBinaryPersistent[T any](val *T, data []byte) error {
	return cbor.NewDecoder(bytes.NewReader(data)).Decode(val)
}

func FilterFieldsByTag[T any](val *T, tag string) T {
	t := reflect.TypeOf(*val)
	if t.Kind() != reflect.Struct {
		panic("only structs accepted")
	}

	filtered := new(T)
	v := reflect.ValueOf(*val)

	for i, fld := range reflect.VisibleFields(t) {
		if _, ok := fld.Tag.Lookup(tag); ok {
			reflect.ValueOf(filtered).Elem().Field(i).Set(v.FieldByIndex(fld.Index))
		}
	}

	return *filtered
}
