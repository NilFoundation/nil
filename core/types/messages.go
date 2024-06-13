package types

type DeployMessage struct {
	ShardId ShardId
	Seqno   Seqno
	Code    Code `ssz-max:"24576"`
}

type AddressSourceData struct {
	DeployMessage
	Salt uint64
}

func NewDeployMessage(data []byte) (*DeployMessage, error) {
	msg := &DeployMessage{}
	if err := msg.UnmarshalSSZ(data); err != nil {
		return nil, err
	}
	return msg, nil
}
