package types

import (
	"errors"
)

type DeployMessage struct {
	ShardId   ShardId
	Seqno     uint64
	PublicKey [PublicKeySize]byte
	Code      Code `ssz-max:"24576"`
}

type AddressSourceData struct {
	DeployMessage
	From Address
	Salt uint64
}

func NewDeployMessage(data []byte) (*DeployMessage, error) {
	var msg DeployMessage

	if err := msg.UnmarshalSSZ(data); err != nil {
		return nil, err
	}

	if l := len(msg.PublicKey); l != PublicKeySize {
		return nil, errors.New("invalid public key size")
	}

	return &msg, nil
}
