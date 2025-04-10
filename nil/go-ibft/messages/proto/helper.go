// Package proto defines the code for protocol buffer
package proto

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

// PayloadNoSig returns marshaled message without signature
func (m *IbftMessage) PayloadNoSig() ([]byte, error) {
	mm, ok := proto.Clone(m).(*IbftMessage)
	if !ok {
		return nil, errors.New("failed to clone message")
	}
	mm.Signature = nil

	raw, err := proto.Marshal(mm)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
