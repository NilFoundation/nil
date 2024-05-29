package shardchain

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var MainPrivateKey *ecdsa.PrivateKey

func init() {
	// All this info should be provided via zerostate / config / etc
	// but for now it's hardcoded for simplicity.
	pubkeyHex := "02eb7216201e65f0a41bc655ada025ad943b79d38aca4d671cbd9875b9604f1ac1"
	pubkey, err := hex.DecodeString(pubkeyHex)
	common.FatalIf(err, sharedLogger, "Failed to prepare main key (decode hex)")

	key, err := crypto.DecompressPubkey(pubkey)
	common.FatalIf(err, sharedLogger, "Failed to prepare main key (unmarshal)")

	keyD := new(big.Int)
	keyD.SetString("29471664811761943693235393363502564971627872515497410365595228231506458150155", 10)
	MainPrivateKey = &ecdsa.PrivateKey{PublicKey: *key, D: keyD}

	common.Require(key.Equal(MainPrivateKey.Public()))
}

func GenerateZeroState(ctx context.Context, es *execution.ExecutionState) error {
	shardId := uint32(es.ShardId)
	mainDeployMsg := &types.DeployMessage{
		ShardId:   shardId,
		Seqno:     0,
		PublicKey: crypto.CompressPubkey(&MainPrivateKey.PublicKey),
	}

	pub := crypto.CompressPubkey(&MainPrivateKey.PublicKey)
	addr := common.PubkeyBytesToAddress(shardId, pub)
	es.CreateAccount(addr)
	es.CreateContract(addr)
	es.SetInitState(addr, mainDeployMsg)

	mainBalance, err := uint256.FromDecimal("1000000000000")
	if err != nil {
		return err
	}

	es.SetBalance(addr, *mainBalance)

	return nil
}
