package ssz

import common "github.com/NilFoundation/nil/common"

type SSZEncodable interface {
	EncodeSSZ([]byte) ([]byte, error)
	EncodingSizeSSZ() int
}

type SSZDecodable interface {
	DecodeSSZ(buf []byte, version int) error
	common.Clonable
}

type Sized interface {
	Static() bool
}

type ObjectSSZ interface {
	SSZEncodable
	SSZDecodable
}

type SizedObjectSSZ interface {
	ObjectSSZ
	Sized
}
