package l2

import (
	"context"
	"math/big"
	"os"
	"path"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	nilcrypto "github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestL2BridgeMessengerABI(t *testing.T) {
	t.Parallel()

	clMock := &client.ClientMock{}
	clMock.GetCodeFunc = func(context.Context, types.Address, any) (types.Code, error) {
		return []byte("code"), nil
	}

	key, _, err := nilcrypto.GenerateKeyPair()
	require.NoError(t, err)

	keyPath := path.Join(os.TempDir(), "test_key.ecdsa")

	require.NoError(t, crypto.SaveECDSA(keyPath, key))
	defer os.Remove(keyPath)

	logger := logging.NewLogger("relayer_l2_contract_wrapper_test")

	wrapper, err := NewL2ContractWrapper(
		t.Context(),
		&ContractConfig{
			Endpoint:            "localhost:8545",
			SmartAccountAddress: "0xDEADBEEF",
			ContractAddress:     "0xC0FFEE",
			PrivateKeyPath:      keyPath,
		},
		clMock,
		logger,
	)
	require.NoError(t, err)

	_, err = wrapper.RelayMessage(
		t.Context(),
		&Event{
			BlockNumber:    1,
			Hash:           ethcommon.Hash{},
			SequenceNumber: 1,
			FeePack: types.FeePack{
				FeeCredit:            types.NewValueFromUint64(1),
				MaxPriorityFeePerGas: types.NewValueFromUint64(1),
				MaxFeePerGas:         types.NewValueFromUint64(1),
			},
			L2Limit:    types.NewValueFromUint64(2),
			Sender:     ethcommon.BigToAddress(big.NewInt(0x111)),
			Target:     ethcommon.BigToAddress(big.NewInt(0x222)),
			Message:    []byte("some very important deposit data"),
			Nonce:      big.NewInt(2),
			Type:       1,
			ExpiryTime: big.NewInt(1),
		},
	)
	require.NoError(t, err)
}
