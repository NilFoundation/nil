package rollupcontract

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// CommitBatch creates blob transaction for `CommitBatch` contract method and sends it on chain.
// If such `batchIndex` is already submitted, returns `nil, ErrBatchAlreadyCommitted`.
func (r *wrapperImpl) CommitBatch(
	ctx context.Context,
	sidecar *ethtypes.BlobTxSidecar,
	batchIndex string,
) error {
	isCommited, err := r.rollupContract.IsBatchCommitted(r.getEthCallOpts(ctx), batchIndex)
	if err != nil {
		return err
	}
	if isCommited {
		return ErrBatchAlreadyCommitted
	}

	publicKeyECDSA, ok := r.privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return errors.New("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	blobTx, err := r.createBlobTx(ctx, sidecar, address, batchIndex)
	if err != nil {
		return err
	}

	keyedTransactor, err := r.getKeyedTransactor()
	if err != nil {
		return err
	}

	signedTx, err := keyedTransactor.Signer(address, blobTx)
	if err != nil {
		return err
	}

	err = r.ethClient.SendTransaction(ctx, signedTx)
	if err != nil {
		return err
	}

	r.logger.Info().
		Hex("txHash", signedTx.Hash().Bytes()).
		Int("gasLimit", int(signedTx.Gas())).
		Int("blobGasLimit", int(signedTx.BlobGas())).
		Int("cost", int(signedTx.Cost().Uint64())).
		Any("blobHashes", signedTx.BlobHashes()).
		Int("blobCount", len(sidecar.Blobs)).
		Msg("commit transaction sent")

	receipt, err := r.waitForReceipt(ctx, signedTx.Hash())
	if err != nil {
		return err
	}
	r.logReceiptDetails(receipt)
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return errors.New("CommitBatch tx failed")
	}

	return nil
}

// ComputeSidecar handles all KZG commitment related computations
func (r *wrapperImpl) PrepareBlobs(
	ctx context.Context, blobs []kzg4844.Blob,
) (*ethtypes.BlobTxSidecar, types.DataProofs, error) {
	sidecar, err := r.computeSidecar(blobs)
	if err != nil {
		return nil, nil, fmt.Errorf("computing sidecar: %w", err)
	}

	dataProofs, err := r.computeDataProofs(ctx, sidecar)
	if err != nil {
		return nil, nil, fmt.Errorf("computing data proofs: %w", err)
	}

	return sidecar, dataProofs, nil
}

func (r *wrapperImpl) computeSidecar(blobs []kzg4844.Blob) (*ethtypes.BlobTxSidecar, error) {
	commitments := make([]kzg4844.Commitment, 0, len(blobs))
	proofs := make([]kzg4844.Proof, 0, len(blobs))

	startTime := time.Now()
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
	r.logger.Info().Dur("elapsedTime", time.Since(startTime)).Int("blobsLen", len(blobs)).Msg("blob proof computed")

	return &ethtypes.BlobTxSidecar{
		Blobs:       blobs,
		Commitments: commitments,
		Proofs:      proofs,
	}, nil
}

func (r *wrapperImpl) computeDataProofs(
	ctx context.Context, sidecar *ethtypes.BlobTxSidecar,
) (types.DataProofs, error) {
	blobHashes := sidecar.BlobHashes()
	dataProofs := make(types.DataProofs, len(blobHashes))
	startTime := time.Now()
	for i, blobHash := range blobHashes {
		point := generatePointFromVersionedHash(blobHash)
		proof, claim, err := kzg4844.ComputeProof(&sidecar.Blobs[i], point)
		if err != nil {
			return nil, fmt.Errorf("failed to generate KZG proof from the blob and point: %w", err)
		}
		dataProofs[i] = encodeDataProof(point, claim, sidecar.Commitments[i], proof)
	}
	r.logger.Info().
		Dur("elapsedTime", time.Since(startTime)).Int("blobsLen", len(blobHashes)).Msg("data proofs computed")

	// to make sure proofs are correct. Not necessary, if other code is not buggy
	if err := r.verifyDataProofs(ctx, sidecar.BlobHashes(), dataProofs); err != nil {
		return nil, fmt.Errorf("generated data proofs verification failed: %w", err)
	}

	return dataProofs, nil
}

func (r *wrapperImpl) verifyDataProofs(
	ctx context.Context,
	hashes []ethcommon.Hash,
	dataProofs types.DataProofs,
) error {
	for i, blobHash := range hashes {
		if err := r.rollupContract.VerifyDataProof(r.getEthCallOpts(ctx), blobHash, dataProofs[i]); err != nil {
			// TODO: make verification return a value.
			// Currently, no way to distinguish network error from verification one
			return fmt.Errorf("proof verification failed for blobHash=%s: %w", blobHash.Hex(), err)
		}
	}
	return nil
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
func (r *wrapperImpl) computeTxParams(ctx context.Context, from ethcommon.Address, blobCount int) (*txParams, error) {
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
func (r *wrapperImpl) createBlobTx(
	ctx context.Context,
	sidecar *ethtypes.BlobTxSidecar,
	from ethcommon.Address,
	batchIndex string,
) (*ethtypes.Transaction, error) {
	txParams, err := r.computeTxParams(ctx, from, len(sidecar.Blobs))
	if err != nil {
		return nil, fmt.Errorf("computing tx params: %w", err)
	}

	abi, err := RollupcontractMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("getting ABI: %w", err)
	}

	data, err := abi.Pack("commitBatch", batchIndex, big.NewInt(int64(len(sidecar.Blobs))))
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
		Sidecar:    sidecar,
	}

	return ethtypes.NewTx(b), nil
}
