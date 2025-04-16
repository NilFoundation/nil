package types

import (
	"slices"

	"github.com/NilFoundation/nil/nil/common"
)

type DeployPayload struct {
	bytes []byte
}

func (dp DeployPayload) Code() Code {
	return Code(dp.bytes[:len(dp.bytes)-common.HashSize])
}

func (dp DeployPayload) Salt() common.Hash {
	return common.Hash(dp.bytes[len(dp.bytes)-common.HashSize:])
}

func (dp DeployPayload) Bytes() []byte {
	return dp.bytes
}

func BuildDeployPayload(code Code, salt common.Hash) DeployPayload {
	code = slices.Clone(code)
	code = append(code, salt.Bytes()...)
	return DeployPayload{code}
}

func ParseDeployPayload(data []byte) *DeployPayload {
	if len(data) < 32 {
		return nil
	}
	dp := DeployPayload{data}
	return &dp
}
