package rollupcontract

import (
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

func ComputeDataProofs(sidecar *ethtypes.BlobTxSidecar) ([][]byte, error) {
	blobHashes := sidecar.BlobHashes()
	dataProofs := make([][]byte, len(blobHashes))
	for i, blobHash := range blobHashes {
		point := generatePointFromVersionedHash(blobHash)
		proof, claim, err := kzg4844.ComputeProof(&sidecar.Blobs[i], point)
		if err != nil {
			return nil, fmt.Errorf("failed to generate KZG proof from the blob and point: %w", err)
		}
		dataProofs[i] = encodeDataProof(point, claim, sidecar.Commitments[i], proof)
	}
	return dataProofs, nil
}

func generatePointFromVersionedHash(versionedHash ethcommon.Hash) kzg4844.Point {
	blsModulo, _ := new(big.Int).SetString(
		"52435875175126190479447740508185965837690552500527637822603658699938581184513", 10)
	pointHash := common.Keccak256Hash(versionedHash[:])

	pointBigInt := new(big.Int).SetBytes(pointHash.Bytes())
	pointBytes := new(big.Int).Mod(pointBigInt, blsModulo).Bytes()
	start := 32 - len(pointBytes)
	var point kzg4844.Point
	copy(point[start:], pointBytes)

	return point
}

func encodeDataProof(
	point kzg4844.Point,
	claim kzg4844.Claim,
	commitment kzg4844.Commitment,
	proof kzg4844.Proof,
) []byte {
	result := make([]byte, 32+32+48+48)

	copy(result[0:32], point[:])
	copy(result[32:64], claim[:])
	copy(result[64:112], commitment[:])
	copy(result[112:160], proof[:])

	return result
}
