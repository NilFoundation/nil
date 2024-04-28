package types

type Code []byte

func (c Code) String() string {
	return string(c)
}
