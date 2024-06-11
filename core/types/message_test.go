package types

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageSign(t *testing.T) {
	t.Parallel()

	from := HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	to := HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F21")

	msg := Message{
		Seqno:    0,
		GasPrice: Uint256{*uint256.NewInt(123)},
		GasLimit: Uint256{*uint256.NewInt(124)},
		From:     from,
		To:       to,
		Value:    Uint256{*uint256.NewInt(125)},
		Data:     Code("qwerty"),
	}

	h, err := msg.SigningHash()
	require.NoError(t, err)
	assert.Equal(t, common.HexToHash("19a91c1790774a7b32a900a54389d797ecb50c1ebf2c4cfb4a3557092c1d7d14"), h)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	err = msg.Sign(key)
	require.NoError(t, err)
	assert.Len(t, msg.Signature, common.SignatureSize)
	assert.True(t, crypto.TransactionSignatureIsValidBytes(msg.Signature[:]))

	pub, err := crypto.SigToPub(h.Bytes(), msg.Signature[:])
	require.NoError(t, err)
	assert.Equal(t, key.PublicKey, *pub)

	pubBytes := crypto.CompressPubkey(pub)
	assert.True(t, crypto.VerifySignature(pubBytes, h.Bytes(), msg.Signature[:64]))
}
