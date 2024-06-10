package execution

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var MainPrivateKey *ecdsa.PrivateKey

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

func (es *ExecutionState) GenerateZeroState(ctx context.Context) error {
	faucetCode, err := contracts.GetCode("Faucet")
	if err != nil {
		return err
	}

	mainDeployMsg := &types.DeployMessage{
		ShardId:   es.ShardId,
		Seqno:     0,
		Code:      faucetCode,
		PublicKey: [types.PublicKeySize]byte(crypto.CompressPubkey(&MainPrivateKey.PublicKey)),
	}

	pub := crypto.CompressPubkey(&MainPrivateKey.PublicKey)
	addr := types.PubkeyBytesToAddress(es.ShardId, pub)
	es.CreateAccount(addr)
	es.CreateContract(addr)
	if err := es.SetInitState(addr, mainDeployMsg); err != nil {
		return err
	}

	mainBalance, err := uint256.FromDecimal("1000000000000")
	if err != nil {
		return err
	}

	es.SetBalance(addr, *mainBalance)

	if es.ShardId == 0 {
		err = es.deployMainWallet(&types.Uint256{Int: *mainBalance})
		if err != nil {
			return err
		}
	}

	return nil
}

func (es *ExecutionState) deployMainWallet(balance *types.Uint256) error {
	code, err := contracts.GetCode("Wallet")
	if err != nil {
		return err
	}

	abi, err := contracts.GetAbi("Wallet")
	if err != nil {
		return err
	}
	pub := crypto.CompressPubkey(&MainPrivateKey.PublicKey)
	args, err := abi.Pack("", pub)
	if err != nil {
		return err
	}
	code = append(code, args...)

	deployMsg := &types.DeployMessage{
		ShardId: es.ShardId,
		Seqno:   0,
		Code:    code,
	}

	es.CreateAccount(types.MainWalletAddress)
	es.CreateContract(types.MainWalletAddress)
	if err := es.SetInitState(types.MainWalletAddress, deployMsg); err != nil {
		return err
	}
	es.SetBalance(types.MainWalletAddress, balance.Int)

	return nil
}
