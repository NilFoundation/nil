package ssz

import common "github.com/NilFoundation/nil/common"

type SSZEncodable interface {
	EncodeSSZ(dst *[]byte) error
	EncodingSizeSSZ() int
}

type SSZDecodable interface {
	common.Clonable
	DecodeSSZ(buf []byte, version int) error
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
