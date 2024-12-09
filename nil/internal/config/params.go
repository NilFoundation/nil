package config

import (
	"errors"

	"github.com/NilFoundation/nil/nil/internal/types"
)

const ValidatorPubkeySize = 33

var ParamsList = []IConfigParam{
	new(ParamValidators),
	new(ParamGasPrice),
}

// This is a workaround for fastssz bug where it doesn't add import of `types` package to generated code.
// Adding this struct solves the issue. It can be removed once something other from `types` package will be used in the
// following structs.
type WorkaroundToImportTypes struct {
	Tmp types.MessageIndex
}

var ErrParamCastFailed = errors.New("input object cannot be cast to Param")

type ParamValidators struct {
	List []ValidatorInfo `json:"list" ssz-max:"4096" yaml:"list"`
}

type ValidatorInfo struct {
	PublicKey         [ValidatorPubkeySize]byte `json:"pubKey" yaml:"pubKey" ssz-size:"33"`
	WithdrawalAddress types.Address             `json:"withdrawalAddress" yaml:"withdrawalAddress"`
}

var _ IConfigParam = new(ParamValidators)

func (p *ParamValidators) Name() string {
	return "curr_validators"
}

func (p *ParamValidators) Accessor() *ParamAccessor {
	return CreateAccessor[ParamValidators]()
}

type ParamGasPrice struct {
	GasPriceScale types.Uint256   `json:"gasPriceScale" yaml:"gasPriceScale"`
	Shards        []types.Uint256 `json:"shards" ssz-max:"4096" yaml:"shards"`
}

var _ IConfigParam = new(ParamGasPrice)

func (p *ParamGasPrice) Name() string {
	return "gas_price"
}

func (p *ParamGasPrice) Accessor() *ParamAccessor {
	return CreateAccessor[ParamGasPrice]()
}

func CreateAccessor[T any, paramPtr IConfigParamPointer[T]]() *ParamAccessor {
	return &ParamAccessor{
		func(c *ConfigAccessorRo) (any, error) { return GetParamRo[T, paramPtr](c) },
		func(c *ConfigAccessor) (any, error) { return GetParam[T, paramPtr](c) },
		func(c *ConfigAccessor, v any) error {
			if param, ok := v.(*T); ok {
				return SetParam[T](c, param)
			}
			return ErrParamCastFailed
		},
		func(v any) ([]byte, error) {
			if param, ok := v.(*T); ok {
				return PackSolidity[T](param)
			}
			return nil, ErrParamCastFailed
		},
		func(data []byte) (any, error) { return UnpackSolidity[T](data) },
	}
}
