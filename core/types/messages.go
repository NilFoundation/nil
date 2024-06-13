package types

import (
	"slices"

	"github.com/NilFoundation/nil/common"
)

type DeployPayload []byte

func (dp DeployPayload) Code() Code {
	return Code(dp[:len(dp)-common.HashSize])
}

func (dp DeployPayload) Bytes() []byte {
	return dp
}

func BuildDeployPayload(code []byte, salt common.Hash) DeployPayload {
	code = slices.Clone(code)
	code = append(code, salt.Bytes()...)
	return code
}

func ParseDeployPayload(data []byte) *DeployPayload {
	if len(data) < 32 {
		return nil
	}
	dp := DeployPayload(data)
	return &dp
}
