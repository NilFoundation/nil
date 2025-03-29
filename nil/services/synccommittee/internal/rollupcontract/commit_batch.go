package rollupcontract

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// CommitBatch creates blob transaction for `CommitBatch` contract method and sends it on chain.
// If such `batchIndex` is already submitted, returns `signedTx, ErrBatchAlreadyCommitted`,
// so `signedTx` could be used later for accessing prepared blobs fields.
func (r *Wrapper) CommitBatch(
	ctx context.Context,
	blobs []kzg4844.Blob,
	batchIndex string,
) (*ethtypes.Transaction, error) {
	callOpts, cancel := r.getEthCallOpts(ctx)
	defer cancel()
	isCommited, err := r.rollupContract.IsBatchCommitted(callOpts, batchIndex)
	if err != nil {
		return nil, err
	}

	publicKeyECDSA, ok := r.privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.requestTimeout)
	defer cancel()

	blobTx, err := r.createBlobTx(ctxWithTimeout, blobs, address, batchIndex)
	if err != nil {
		return nil, err
	}

	keyedTransactor, err := bind.NewKeyedTransactorWithChainID(r.privateKey, r.chainID)
	if err != nil {
		return nil, fmt.Errorf("creating keyed transactor with chain ID: %w", err)
	}

	signedTx, err := keyedTransactor.Signer(address, blobTx)
	if err != nil {
		return nil, err
	}

	if isCommited {
		return signedTx, ErrBatchAlreadyCommitted
	}

	err = r.ethClient.SendTransaction(ctxWithTimeout, signedTx)
	if err != nil {
		return nil, err
	}

	return signedTx, nil
}

// computeSidecar handles all KZG commitment related computations
func computeSidecar(blobs []kzg4844.Blob) (*ethtypes.BlobTxSidecar, error) {
	commitments := make([]kzg4844.Commitment, 0, len(blobs))
	proofs := make([]kzg4844.Proof, 0, len(blobs))

	for _, blob := range blobs {
		commitment, err := kzg4844.BlobToCommitment(&blob)
		if err != nil {
			return nil, fmt.Errorf("computing commitment: %w", err)
		}

		proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
		if err != nil {
			return nil, fmt.Errorf("computing proof: %w", err)
		}

		commitments = append(commitments, commitment)
		proofs = append(proofs, proof)
	}

	return &ethtypes.BlobTxSidecar{
		Blobs:       blobs,
		Commitments: commitments,
		Proofs:      proofs,
	}, nil
}

// txParams holds all the Ethereum transaction related parameters
type txParams struct {
	Nonce      uint64
	GasTipCap  *big.Int
	GasFeeCap  *big.Int
	BlobFeeCap *big.Int
	Gas        uint64
}

// computeTxParams fetches and computes all necessary transaction parameters
func (r *Wrapper) computeTxParams(ctx context.Context, from ethcommon.Address, blobCount int) (*txParams, error) {
	nonce, err := r.ethClient.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("getting nonce: %w", err)
	}

	gasTipCap, err := r.ethClient.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggesting gas tip cap: %w", err)
	}

	head, err := r.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("getting header: %w", err)
	}

	const basefeeWiggleMultiplier = 2
	gasFeeCap := new(big.Int).Add(
		gasTipCap,
		new(big.Int).Mul(head.BaseFee, big.NewInt(basefeeWiggleMultiplier)),
	)

	if gasFeeCap.Cmp(gasTipCap) < 0 {
		return nil, fmt.Errorf("maxFeePerGas (%v) < maxPriorityFeePerGas (%v)", gasFeeCap, gasTipCap)
	}

	blobFee := eip4844.CalcBlobFee(*head.ExcessBlobGas)
	gas := ethparams.BlobTxBlobGasPerBlob * uint64(blobCount)

	return &txParams{
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		BlobFeeCap: blobFee,
		Gas:        gas,
	}, nil
}

// createBlobTx creates a new blob transaction using the computed blob data and transaction parameters
func (r *Wrapper) createBlobTx(
	ctx context.Context,
	blobs []kzg4844.Blob,
	from ethcommon.Address,
	batchIndex string,
) (*ethtypes.Transaction, error) {
	startTime := time.Now()
	sidecar, err := computeSidecar(blobs)
	if err != nil {
		return nil, fmt.Errorf("computing blob data: %w", err)
	}
	r.logger.Info().Dur("elapsedTime", time.Since(startTime)).Int("blobsLen", len(blobs)).Msg("blob proof computed")

	txParams, err := r.computeTxParams(ctx, from, len(blobs))
	if err != nil {
		return nil, fmt.Errorf("computing tx params: %w", err)
	}

	abi, err := RollupcontractMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("getting ABI: %w", err)
	}

	data, err := abi.Pack("commitBatch", batchIndex, big.NewInt(int64(len(blobs))))
	if err != nil {
		return nil, fmt.Errorf("packing ABI data: %w", err)
	}

	b := &ethtypes.BlobTx{
		ChainID:    uint256.MustFromBig(r.chainID),
		Nonce:      txParams.Nonce,
		GasTipCap:  uint256.MustFromBig(txParams.GasTipCap),
		GasFeeCap:  uint256.MustFromBig(txParams.GasFeeCap),
		Gas:        txParams.Gas,
		To:         r.contractAddress,
		Value:      uint256.NewInt(0),
		Data:       data,
		AccessList: nil,
		BlobFeeCap: uint256.MustFromBig(txParams.BlobFeeCap),
		BlobHashes: sidecar.BlobHashes(),
		Sidecar: &ethtypes.BlobTxSidecar{
			Blobs:       sidecar.Blobs,
			Commitments: sidecar.Commitments,
			Proofs:      sidecar.Proofs,
		},
	}

	return ethtypes.NewTx(b), nil
}
