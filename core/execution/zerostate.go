package execution

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"gopkg.in/yaml.v3"
)

var MainPrivateKey *ecdsa.PrivateKey

const DefaultZeroStateConfig = `
contracts:
- name: Faucet
  value: 1000000000000
  shard: 1
  contract: Faucet
- name: MainWallet
  address: 0x0000111111111111111111111111111111111111
  value: 100000000000000
  contract: Wallet
  ctorArgs: [0x02eb7216201e65f0a41bc655ada025ad943b79d38aca4d671cbd9875b9604f1ac1]
`

func init() {
	// All this info should be provided via zerostate / config / etc
	// but for now it's hardcoded for simplicity.
	pubkeyHex := "02eb7216201e65f0a41bc655ada025ad943b79d38aca4d671cbd9875b9604f1ac1"
	pubkey, err := hex.DecodeString(pubkeyHex)
	check.PanicIfErr(err)

	key, err := crypto.DecompressPubkey(pubkey)
	check.PanicIfErr(err)

	keyD := new(big.Int)
	keyD.SetString("29471664811761943693235393363502564971627872515497410365595228231506458150155", 10)
	MainPrivateKey = &ecdsa.PrivateKey{PublicKey: *key, D: keyD}

	check.PanicIfNot(key.Equal(MainPrivateKey.Public()))
}

type ContractDescr struct {
	Name     string         `yaml:"name"`
	Address  *types.Address `yaml:"address,omitempty"`
	Value    *types.Uint256 `yaml:"value"`
	Shard    types.ShardId  `yaml:"shard,omitempty"`
	Contract string         `yaml:"contract"`
	CtorArgs []any          `yaml:"ctorArgs,omitempty"`
}

type ZeroStateConfig struct {
	Contracts []*ContractDescr `yaml:"contracts"`
}

func (es *ExecutionState) GenerateZeroState(configYaml string) error {
	var config ZeroStateConfig
	err := yaml.Unmarshal([]byte(configYaml), &config)
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
					pub := crypto.CompressPubkey(&MainPrivateKey.PublicKey)
					args = append(args, pub)
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

		mainDeployMsg := &types.DeployMessage{
			ShardId: es.ShardId,
			Seqno:   0,
			Code:    code,
		}

		es.CreateAccount(addr)
		es.CreateContract(addr)
		if err := es.SetInitState(addr, mainDeployMsg); err != nil {
			return err
		}

		var value types.Uint256
		if contract.Value != nil {
			value = *contract.Value
		} else {
			value = *types.NewUint256(0)
		}

		es.SetBalance(addr, value.Int)
	}
	return nil
}
