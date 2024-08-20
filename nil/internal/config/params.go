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
	return &ParamAccessor{
		func(c *ConfigAccessor) (any, error) { return GetParam[ParamValidators](c) },
		func(c *ConfigAccessor, v any) error {
			if param, ok := v.(*ParamValidators); ok {
				return SetParam[ParamValidators](c, param)
			}
			return ErrParamCastFailed
		},
		func(c *ConfigAccessor, v any) ([]byte, error) {
			if param, ok := v.(*ParamValidators); ok {
				return PackSolidity[ParamValidators](param)
			}
			return nil, ErrParamCastFailed
		},
		func(c *ConfigAccessor, data []byte) (any, error) { return UnpackSolidity[ParamValidators](data) },
	}
}

type ParamGasPrice struct {
	GasPriceScale types.Uint256 `json:"gasPriceScale" yaml:"gasPriceScale"`
}

var _ IConfigParam = new(ParamGasPrice)

func (p *ParamGasPrice) Name() string {
	return "gas_price"
}

func (p *ParamGasPrice) Accessor() *ParamAccessor {
	return &ParamAccessor{
		func(c *ConfigAccessor) (any, error) { return GetParam[ParamGasPrice](c) },
		func(c *ConfigAccessor, v any) error {
			if param, ok := v.(*ParamGasPrice); ok {
				return SetParam[ParamGasPrice](c, param)
			}
			return ErrParamCastFailed
		},
		func(c *ConfigAccessor, v any) ([]byte, error) {
			if param, ok := v.(*ParamGasPrice); ok {
				return PackSolidity[ParamGasPrice](param)
			}
			return nil, ErrParamCastFailed
		},
		func(c *ConfigAccessor, data []byte) (any, error) { return UnpackSolidity[ParamGasPrice](data) },
	}
}
