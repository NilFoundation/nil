package serialization

type EncodedData = []byte

func EncodeContainer[T NilMarshaler](container []T) ([]EncodedData, error) {
	if len(container) == 0 {
		return nil, nil
	}

	result := make([]EncodedData, 0, len(container))
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
		NilUnmarshaler
	},
	S any,
](dataContainer []EncodedData) ([]*S, error) {
	if len(dataContainer) == 0 {
		return nil, nil
	}

	result := make([]*S, 0, len(dataContainer))
	for _, rawData := range dataContainer {
		decoded := new(S)
		if err := T(decoded).UnmarshalNil(rawData); err != nil {
			return nil, err
		}
		result = append(result, decoded)
	}
	return result, nil
}
