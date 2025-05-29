package types

import (
	"encoding/json"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionSign(t *testing.T) {
	t.Parallel()

	to := HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F21")

	txn := ExternalTransaction{
		Seqno: 0,
		To:    to,
		Data:  Code("qwerty"),
	}

	h, err := txn.SigningHash()
	require.NoError(t, err)
	assert.Equal(t, common.HexToHash("cc0edb9f217689230d266603d9c3e79118aedf22c182dedf1aa4577935d35df8"), h)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	err = txn.Sign(key)
	require.NoError(t, err)
	assert.Len(t, txn.AuthData, common.SignatureSize)

	pub, err := crypto.SigToPub(h.Bytes(), txn.AuthData[:])
	require.NoError(t, err)
	assert.Equal(t, key.PublicKey, *pub)

	pubBytes := crypto.FromECDSAPub(pub)
	assert.True(t, crypto.VerifySignature(pubBytes, h.Bytes(), txn.AuthData[:64]))
}

func TestTransactionFlagsJson(t *testing.T) {
	t.Parallel()

	m := NewTransactionFlags(TransactionFlagInternal, TransactionFlagRefund)
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var m2 TransactionFlags
	require.NoError(t, json.Unmarshal(data, &m2))
	require.Equal(t, m, m2)

	m = NewTransactionFlags(
		TransactionFlagInternal, TransactionFlagRefund, TransactionFlagDeploy, TransactionFlagBounce)
	data, err = json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &m2))
	require.Equal(t, m, m2)

	m = NewTransactionFlags()
	data, err = json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &m2))
	require.Equal(t, m, m2)
}
