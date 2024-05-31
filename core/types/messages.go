package types

import (
	"errors"

	"github.com/NilFoundation/nil/common"
)

type DeployMessage struct {
	ShardId   ShardId
	Seqno     uint64
	PublicKey []byte `ssz-max:"33"`
	Data      Code   `ssz-max:"24576"`
	Code      Code   `ssz-max:"24576"`
}

func NewDeployMessage(data []byte) (*DeployMessage, error) {
	var msg DeployMessage

	if err := msg.UnmarshalSSZ(data); err != nil {
		return nil, err
	}

	if l := len(msg.PublicKey); l != common.PublicKeySize && l != 0 {
		return nil, errors.New("invalid public key size")
	}

	return &msg, nil
}
