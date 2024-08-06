package common

const (
	BoolSize = 1
	// expected length of bool
	Uint64Size = 8
	// expected length of uint64
	Bits256Size = 32
	// expected length of uint256
	PeerIDSize = 64
	// Hash is the expected length of the hash (in bytes)
	HashSize = 32
	// SignatureSize indicates the byte length required to carry a signature with recovery id.
	SignatureSize = 65 // 64 bytes ECDSA signature + 1 byte recovery id
	// expected length of Bytes96 (signature)
	Bytes96Size = 96
	// expected length of Bytes48 (bls public key and such)
	Bytes48Size = 48
	// expected length of Bytes64 (sync committee bits)
	Bytes64Size = 64
	// expected length of Bytes48 (beacon domain and such)
	Bytes4Size = 4
	// BlockNumberLen length of uint64 big endian
	BlockNumSize = 8
	// Ts TimeStamp (BlockNum, TxNum or any other uint64 equivalent of Time)
	TsSize = 8
	// Incarnation length of uint64 for contract incarnations
	IncarnationSize = 8
)
