package sszx

import (
	"github.com/NilFoundation/nil/nil/internal/serialization"
)

type SSZEncodedData = []byte

func EncodeContainer[T serialization.NilMarshaler](container []T) ([]SSZEncodedData, error) {
	result := make([]SSZEncodedData, 0, len(container))
	for _, data := range container {
		content, err := data.MarshalNil()
		if err != nil {
			return nil, err
		}
		result = append(result, content)
	}
	return result, nil
}

func DecodeContainer[
	T interface {
		~*S
		serialization.NilUnmarshaler
	},
	S any,
](dataContainer []SSZEncodedData) ([]*S, error) {
	result := make([]*S, 0, len(dataContainer))
	for _, sszData := range dataContainer {
		decoded := new(S)
		if err := T(decoded).UnmarshalNil(sszData); err != nil {
			return []*S{}, err
		}
		result = append(result, decoded)
	}
	return result, nil
}
