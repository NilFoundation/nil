package proto

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

func (m *Message) PayloadNoSig() ([]byte, error) {
	mm, ok := proto.Clone(m).(*Message)
	if !ok {
		return nil, errors.New("unable to cast message type")
	}

	mm.Signature = nil

	raw, err := proto.Marshal(mm)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
