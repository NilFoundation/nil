package rollupcontract

import (
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

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
