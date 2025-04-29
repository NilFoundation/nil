package serialization

type NilMarshaler interface {
	MarshalNil() ([]byte, error)
}

type NilUnmarshaler interface {
	UnmarshalNil(buf []byte) error
}
