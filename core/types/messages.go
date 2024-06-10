package types

type DeployMessage struct {
	ShardId   ShardId
	Seqno     Seqno
	PublicKey [PublicKeySize]byte
	Code      Code `ssz-max:"24576"`
}

type AddressSourceData struct {
	DeployMessage
	From Address
	Salt uint64
}

func NewDeployMessage(data []byte) (*DeployMessage, error) {
	msg := &DeployMessage{}
	if err := msg.UnmarshalSSZ(data); err != nil {
		return nil, err
	}
	return msg, nil
}
