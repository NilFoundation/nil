package tracer

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type EcdsaSign struct {
	hash       common.Hash
	r          types.Uint256
	s          types.Uint256
	v          byte
	pubKeyX    types.Uint256
	pubKeyY    types.Uint256
}

type EcdsaTracer struct {
	signs []EcdsaSign
}

func NewEcdsaTracer() *EcdsaTracer {
	return &EcdsaTracer{}
}

func (et *EcdsaTracer) TraceTx(tx *types.Transaction) error {
	if !tx.IsExternal() {
		return nil
	}

	if len(tx.Signature) != 65 {
		return fmt.Errorf("Unsupported signature len = %d, expect 65", len(tx.Signature))
	}

	hash, err := tx.SigningHash()
	if err != nil {
		return err
	}

	pk, err := crypto.SigToPub(hash.Bytes(), tx.Signature)
	if err != nil {
		return err
	}

	sign := EcdsaSign{
		hash:    hash,
		r:       *types.NewUint256FromBytes(tx.Signature[0:32]),
		s:       *types.NewUint256FromBytes(tx.Signature[32:64]),
		v:       tx.Signature[64],
	}
	sign.pubKeyX.SetFromBig(pk.X)
	sign.pubKeyY.SetFromBig(pk.Y)
	et.signs = append(et.signs, sign)

	return nil
}

func (et *EcdsaTracer) Finalize() []EcdsaSign {
	return et.signs
}
