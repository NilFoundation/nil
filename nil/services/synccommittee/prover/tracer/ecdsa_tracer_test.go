package tracer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func traceExtTxSignature(t *testing.T, tracer *EcdsaTracer) []EcdsaSign {
	t.Helper()

	to := types.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F21")

	txn := types.ExternalTransaction{
		Seqno: 0,
		To:    to,
		Data:  types.Code("qwerty"),
	}

	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	err = txn.Sign(privateKey)
	require.NoError(t, err)

	err = tracer.TraceTx(txn.ToTransaction())
	require.NoError(t, err)

	return tracer.Finalize()
}

func TestEcdsaTracer_HandlesExtTxSignarure(t *testing.T) {
	t.Parallel()
	tracer := &EcdsaTracer{}

	signs := traceExtTxSignature(t, tracer)
	require.Len(t, signs, 1)
	sign := signs[0]
	fmt.Println(hexutil.Encode(sign.hash.Bytes()))
	fmt.Println(hexutil.Encode(sign.r.Bytes()))
	fmt.Println(hexutil.Encode(sign.s.Bytes()))
	fmt.Println(sign.v)
	fmt.Println(hexutil.Encode(sign.pubKeyX.Bytes()))
	fmt.Println(hexutil.Encode(sign.pubKeyY.Bytes()))
}