package types

import (
	"encoding/json"
	"testing"

	"github.com/NilFoundation/nil/common"
	nilcrypto "github.com/NilFoundation/nil/core/crypto"
	"github.com/ethereum/go-ethereum/crypto"
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
	assert.Equal(t, common.HexToHash("010d6f0d844c4bf322002957ab4a372dad725dcc85d3236be83ac1b15ebf6eeb"), h)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	err = msg.Sign(key)
	require.NoError(t, err)
	assert.Len(t, msg.AuthData, common.SignatureSize)
	assert.True(t, nilcrypto.TransactionSignatureIsValidBytes(msg.AuthData[:]))

	pub, err := crypto.SigToPub(h.Bytes(), msg.AuthData[:])
	require.NoError(t, err)
	assert.Equal(t, key.PublicKey, *pub)

	pubBytes := crypto.CompressPubkey(pub)
	assert.True(t, crypto.VerifySignature(pubBytes, h.Bytes(), msg.AuthData[:64]))
}

func TestMessageFlagsJson(t *testing.T) {
	t.Parallel()

	m := NewMessageFlags(MessageFlagInternal, MessageFlagRefund)
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var m2 MessageFlags
	require.NoError(t, json.Unmarshal(data, &m2))
	require.Equal(t, m, m2)

	m = NewMessageFlags(MessageFlagInternal, MessageFlagRefund, MessageFlagDeploy, MessageFlagBounce)
	data, err = json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &m2))
	require.Equal(t, m, m2)

	m = NewMessageFlags()
	data, err = json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &m2))
	require.Equal(t, m, m2)
}
