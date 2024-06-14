package types

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageSign(t *testing.T) {
	t.Parallel()

	to := HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F21")

	msg := ExternalMessage{
		Seqno: 0,
		To:    to,
		Data:  Code("qwerty"),
	}

	h, err := msg.SigningHash()
	require.NoError(t, err)
	assert.Equal(t, common.HexToHash("19185e8da5495483b366ce3ab5bb37de552fb3576aac2b8ce9daf07ab393a46c"), h)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	err = msg.Sign(key)
	require.NoError(t, err)
	assert.Len(t, msg.AuthData, common.SignatureSize)
	assert.True(t, crypto.TransactionSignatureIsValidBytes(msg.AuthData[:]))

	pub, err := crypto.SigToPub(h.Bytes(), msg.AuthData[:])
	require.NoError(t, err)
	assert.Equal(t, key.PublicKey, *pub)

	pubBytes := crypto.CompressPubkey(pub)
	assert.True(t, crypto.VerifySignature(pubBytes, h.Bytes(), msg.AuthData[:64]))
}
