package execution

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/holiman/uint256"
)

var MainPrivateKey *ecdsa.PrivateKey

func init() {
	// All this info should be provided via zerostate / config / etc
	// but for now it's hardcoded for simplicity.
	pubkeyHex := "02eb7216201e65f0a41bc655ada025ad943b79d38aca4d671cbd9875b9604f1ac1"
	pubkey, err := hex.DecodeString(pubkeyHex)
	common.FatalIf(err, logger, "Failed to prepare main key (decode hex)")

	key, err := crypto.DecompressPubkey(pubkey)
	common.FatalIf(err, logger, "Failed to prepare main key (unmarshal)")

	keyD := new(big.Int)
	keyD.SetString("29471664811761943693235393363502564971627872515497410365595228231506458150155", 10)
	MainPrivateKey = &ecdsa.PrivateKey{PublicKey: *key, D: keyD}

	common.Require(key.Equal(MainPrivateKey.Public()))
}

//go:generate solc --combined-json abi,bin ./contracts.sol -o contracts

func obtainContractsPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("Failed to obtain current file")
	}
	return filepath.Abs(filepath.Join(filepath.Dir(currentFile), "./contracts/combined.json"))
}

func obtainContracts() (map[string]*compiler.Contract, error) {
	contractsPath, err := obtainContractsPath()
	if err != nil {
		return nil, err
	}
	contractsFile, err := os.Open(contractsPath)
	if err != nil {
		return nil, err
	}
	contractsJSON, err := io.ReadAll(contractsFile)
	if err != nil {
		return nil, err
	}
	return solc.ParseCombinedJSON(contractsJSON)
}

func (es *ExecutionState) GenerateZeroState(ctx context.Context) error {
	contracts, err := obtainContracts()
	if err != nil {
		return err
	}
	faucetContract := contracts["Faucet"]

	mainDeployMsg := &types.DeployMessage{
		ShardId:   es.ShardId,
		Seqno:     0,
		Code:      hexutil.FromHex(faucetContract.Code),
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

	return nil
}
