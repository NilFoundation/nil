package execution

import (
	"crypto/ecdsa"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	nilcrypto "github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gopkg.in/yaml.v3"
)

var (
	MainPrivateKey *ecdsa.PrivateKey
	MainPublicKey  []byte
)

var DefaultZeroStateConfig string

func init() {
	var err error
	MainPrivateKey, MainPublicKey, err = nilcrypto.GenerateKeyPair()
	check.PanicIfErr(err)

	DefaultZeroStateConfig = fmt.Sprintf(`
contracts:
- name: Faucet
  address: %s
  value: 1000000000000
  contract: Faucet
- name: MainWallet
  address: %s
  value: 100000000000000
  contract: Wallet
  ctorArgs: [%s]
`, types.FaucetAddress.Hex(), types.MainWalletAddress.Hex(), hexutil.Encode(MainPublicKey))
}

type ContractDescr struct {
	Name     string         `yaml:"name"`
	Address  *types.Address `yaml:"address,omitempty"`
	Value    *types.Uint256 `yaml:"value"`
	Shard    types.ShardId  `yaml:"shard,omitempty"`
	Contract string         `yaml:"contract"`
	CtorArgs []any          `yaml:"ctorArgs,omitempty"`
}

type MainKeys struct {
	MainPrivateKey string `yaml:"mainPrivateKey"`
	MainPublicKey  string `yaml:"mainPublicKey"`
}

type ZeroStateConfig struct {
	Contracts []*ContractDescr `yaml:"contracts"`
}

func DumpMainKeys(fname string) error {
	keys := MainKeys{"0x" + nilcrypto.PrivateKeyToEthereumFormat(MainPrivateKey), hexutil.Encode(MainPublicKey)}

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

func LoadMainKeys(fname string) error {
	var keys MainKeys

	data, err := os.ReadFile(fname)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &keys); err != nil {
		return err
	}
	MainPrivateKey, err = crypto.HexToECDSA(keys.MainPrivateKey[2:])
	if err != nil {
		return err
	}
	MainPublicKey, err = hexutil.Decode(keys.MainPublicKey)
	return err
}

func (c *ZeroStateConfig) FindContractByName(name string) *ContractDescr {
	for _, contract := range c.Contracts {
		if contract.Name == name {
			return contract
		}
	}
	return nil
}

func (c *ZeroStateConfig) GetContractAddress(name string) *types.Address {
	contract := c.FindContractByName(name)
	if contract != nil {
		return contract.Address
	}
	return nil
}

func ParseZeroStateConfig(configYaml string) (*ZeroStateConfig, error) {
	var config ZeroStateConfig
	err := yaml.Unmarshal([]byte(configYaml), &config)
	return &config, err
}

func (es *ExecutionState) GenerateZeroState(configYaml string) error {
	config, err := ParseZeroStateConfig(configYaml)
	if err != nil {
		return err
	}

	for _, contract := range config.Contracts {
		code, err := contracts.GetCode(contract.Contract)
		if err != nil {
			return err
		}
		var addr types.Address
		if contract.Address != nil {
			addr = *contract.Address
		} else {
			addr = types.CreateAddress(contract.Shard, code)
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
				switch {
				case arg == "MainPublicKey":
					args = append(args, MainPublicKey)
				case arg[:2] == "0x":
					args = append(args, hexutil.FromHex(arg))
				default:
					return fmt.Errorf("unknown constructor argument string pattern: %s", arg)
				}
			default:
				return fmt.Errorf("unsupported constructor argument type: %T", arg)
			}
		}
		argsPacked, err := abi.Pack("", args...)
		if err != nil {
			return err
		}
		code = append(code, argsPacked...)

		mainDeployMsg := &types.Message{
			Internal: true,
			Seqno:    0,
			Data:     code,
		}

		if err := es.CreateAccount(addr); err != nil {
			return err
		}
		if err := es.CreateContract(addr); err != nil {
			return err
		}
		if err := es.SetInitState(addr, mainDeployMsg); err != nil {
			return err
		}

		var value types.Uint256
		if contract.Value != nil {
			value = *contract.Value
		} else {
			value = *types.NewUint256(0)
		}

		if err := es.SetBalance(addr, value.Int); err != nil {
			return err
		}
		logger.Debug().Str("name", contract.Name).Stringer("address", addr).Msg("Created zero state contract")
	}
	return nil
}
