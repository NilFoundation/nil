package execution

import (
	"crypto/ecdsa"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gopkg.in/yaml.v3"
)

type ContractDescr struct {
	Name     string        `yaml:"name"`
	Address  types.Address `yaml:"address,omitempty"`
	Value    types.Value   `yaml:"value"`
	Shard    types.ShardId `yaml:"shard,omitempty"`
	Contract string        `yaml:"contract"`
	CtorArgs []any         `yaml:"ctorArgs,omitempty"`
}

type MainKeys struct {
	MainPrivateKey hexutil.Bytes `yaml:"mainPrivateKey"`
	MainPublicKey  hexutil.Bytes `yaml:"mainPublicKey"`
}

type ConfigParams struct {
	Validators config.ParamValidators `yaml:"validators,omitempty"`
	GasPrice   config.ParamGasPrice   `yaml:"gasPrice"`
}

type ZeroStateConfig struct {
	ConfigParams ConfigParams     `yaml:"config,omitempty"`
	Contracts    []*ContractDescr `yaml:"contracts"`
}

func CreateDefaultZeroStateConfig(mainPublicKey []byte) (*ZeroStateConfig, error) {
	smartAccountValue, err := types.NewValueFromDecimal("100000000000000000000000000000000000000000000000000")
	if err != nil {
		return nil, err
	}
	faucetValue := smartAccountValue.Mul(types.NewValueFromUint64(uint64(2)))
	tokenValue, err := types.NewValueFromDecimal("100000000000000000000000000000000000000000000000000")
	if err != nil {
		return nil, err
	}
	zeroStateConfig := &ZeroStateConfig{
		Contracts: []*ContractDescr{
			{
				Name:     "MainSmartAccount",
				Contract: "SmartAccount",
				Address:  types.MainSmartAccountAddress,
				Value:    smartAccountValue,
				CtorArgs: []any{hexutil.Encode(mainPublicKey)},
			},
			{Name: "Faucet", Contract: "Faucet", Address: types.FaucetAddress, Value: faucetValue},
			{Name: "EthFaucet", Contract: "FaucetToken", Address: types.EthFaucetAddress, Value: tokenValue},
			{Name: "UsdtFaucet", Contract: "FaucetToken", Address: types.UsdtFaucetAddress, Value: tokenValue},
			{Name: "BtcFaucet", Contract: "FaucetToken", Address: types.BtcFaucetAddress, Value: tokenValue},
			{Name: "UsdcFaucet", Contract: "FaucetToken", Address: types.UsdcFaucetAddress, Value: tokenValue},
			{
				Name:     "L1BlockInfo",
				Contract: "system/L1BlockInfo",
				Address:  types.L1BlockInfoAddress,
				Value:    types.Value0,
			},
			{
				Name:     "Governance",
				Contract: "system/Governance",
				Address:  types.GovernanceAddress,
				Value:    smartAccountValue,
				CtorArgs: []any{hexutil.Encode(mainPublicKey)},
			},
		},
	}
	return zeroStateConfig, nil
}

func (cfg *ZeroStateConfig) GetValidators() []config.ListValidators {
	return cfg.ConfigParams.Validators.Validators
}

func DumpMainKeys(fname string, mainPrivateKey *ecdsa.PrivateKey) error {
	mainPublicKey := crypto.CompressPubkey(&mainPrivateKey.PublicKey)
	keys := MainKeys{crypto.FromECDSA(mainPrivateKey), mainPublicKey}

	data, err := yaml.Marshal(&keys)
	if err != nil {
		return err
	}

	file, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func LoadMainKeys(fname string) (*ecdsa.PrivateKey, error) {
	var keys MainKeys

	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	mainPrivateKey, err := crypto.ToECDSA(keys.MainPrivateKey)
	if err != nil {
		return nil, err
	}
	return mainPrivateKey, err
}

func (c *ZeroStateConfig) FindContractByName(name string) *ContractDescr {
	for _, contract := range c.Contracts {
		if contract.Name == name {
			return contract
		}
	}
	return nil
}

func (c *ZeroStateConfig) GetContractAddress(name string) types.Address {
	contract := c.FindContractByName(name)
	if contract != nil {
		return contract.Address
	}
	return types.EmptyAddress
}

func (es *ExecutionState) GenerateZeroState(stateConfig *ZeroStateConfig) error {
	var err error

	if es.ShardId == types.MainShardId {
		cfgAccessor := es.GetConfigAccessor()
		config.InitParams(cfgAccessor)
		err = config.SetParamValidators(cfgAccessor, &stateConfig.ConfigParams.Validators)
		if err != nil {
			return err
		}
		err = config.SetParamGasPrice(cfgAccessor, &stateConfig.ConfigParams.GasPrice)
		if err != nil {
			return err
		}
	}

	if len(stateConfig.ConfigParams.GasPrice.Shards) != 0 {
		check.PanicIfNot(len(stateConfig.ConfigParams.GasPrice.Shards) > int(es.ShardId))
		es.GasPrice = types.Value{Uint256: &stateConfig.ConfigParams.GasPrice.Shards[es.ShardId]}
	} else {
		es.GasPrice = types.DefaultGasPrice
	}

	for _, contract := range stateConfig.Contracts {
		code, err := contracts.GetCode(contract.Contract)
		if err != nil {
			return err
		}
		var addr types.Address
		if contract.Address != types.EmptyAddress {
			addr = contract.Address
		} else {
			addr = types.CreateAddress(contract.Shard, types.BuildDeployPayload(code, common.EmptyHash))
		}

		if addr.ShardId() != es.ShardId {
			continue
		}

		abi, err := contracts.GetAbi(contract.Contract)
		if err != nil {
			return err
		}

		args := make([]any, 0)
		for _, arg := range contract.CtorArgs {
			switch arg := arg.(type) {
			case string:
				if arg[:2] != "0x" {
					return fmt.Errorf("unknown constructor argument string pattern: %s", arg)
				}
				args = append(args, hexutil.FromHex(arg))
			default:
				args = append(args, arg)
			}
		}
		argsPacked, err := abi.Pack("", args...)
		if err != nil {
			return fmt.Errorf("[ZeroState] ctorArgs pack failed: %w", err)
		}
		code = append(code, argsPacked...)

		mainDeployTxn := &types.Transaction{
			TransactionDigest: types.TransactionDigest{
				Flags:        types.NewTransactionFlags(types.TransactionFlagInternal),
				Seqno:        0,
				Data:         code,
				MaxFeePerGas: types.MaxFeePerGasDefault,
			},
		}

		if err := es.CreateAccount(addr); err != nil {
			return err
		}
		if err := es.CreateContract(addr); err != nil {
			return err
		}
		if err := es.SetBalance(addr, contract.Value); err != nil {
			return err
		}
		if err := es.SetInitState(addr, mainDeployTxn); err != nil {
			return err
		}

		es.logger.Debug().Str("name", contract.Name).Stringer("address", addr).Msg("Created zero state contract")
	}
	return nil
}
