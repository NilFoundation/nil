package keys

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"gopkg.in/yaml.v3"
)

var Logger = logging.NewLogger("keys")

type dumpedValidatorKey struct {
	PrivateKey hexutil.Bytes `yaml:"privateKey"`
	PublicKey  hexutil.Bytes `yaml:"publicKey"`
}

type dumpedValidatorKeys struct {
	Keys map[string]dumpedValidatorKey `yaml:"keys"`
}

type ValidatorKeysManager struct {
	validatorKeysPath string
	nShards           uint32
	keys              []*ecdsa.PrivateKey
	init              bool
}

func NewValidatorKeyManager(validatorKeysPath string, nShards uint32) *ValidatorKeysManager {
	return &ValidatorKeysManager{
		validatorKeysPath: validatorKeysPath,
		nShards:           nShards,
		keys:              make([]*ecdsa.PrivateKey, 0, nShards),
	}
}

func (v *ValidatorKeysManager) generateKeys() error {
	for range v.nShards {
		key, err := ecdsa.GenerateKey(gethcrypto.S256(), rand.Reader)
		if err != nil {
			return err
		}
		v.keys = append(v.keys, key)
	}
	return nil
}

const filePermissions = 0o644

func (v *ValidatorKeysManager) dumpKeys() error {
	dumpedKeys := make(map[string]dumpedValidatorKey)
	for i, key := range v.keys {
		privKey := gethcrypto.FromECDSA(key)
		pubKey := gethcrypto.FromECDSAPub(&key.PublicKey)
		dumpedKeys[strconv.Itoa(i)] = dumpedValidatorKey{
			PrivateKey: privKey,
			PublicKey:  pubKey,
		}
	}

	data, err := yaml.Marshal(dumpedValidatorKeys{Keys: dumpedKeys})
	if err != nil {
		return err
	}
	return os.WriteFile(v.validatorKeysPath, data, filePermissions)
}

func (v *ValidatorKeysManager) loadKeys() error {
	Logger.Info().Msgf("Loading keys from path: %s", v.validatorKeysPath)
	data, err := os.ReadFile(v.validatorKeysPath)
	if err != nil {
		return err
	}

	dumpedKeys := &dumpedValidatorKeys{}
	if err := yaml.Unmarshal(data, dumpedKeys); err != nil {
		return err
	}

	if len(dumpedKeys.Keys) != int(v.nShards) {
		return errors.New("number of keys mismatch")
	}

	for i := range v.nShards {
		privKey, err := gethcrypto.ToECDSA(dumpedKeys.Keys[strconv.Itoa(int(i))].PrivateKey)
		if err != nil {
			return err
		}
		pubKey, err := gethcrypto.UnmarshalPubkey(dumpedKeys.Keys[strconv.Itoa(int(i))].PublicKey)
		if err != nil {
			return err
		}
		if !pubKey.Equal(&privKey.PublicKey) {
			return errors.New("public key mismatch")
		}
		v.keys = append(v.keys, privKey)
	}
	return nil
}

// This functions initializes keys for all shards by loading them from the file if it exists,
// or generating new keys and saving them to the file.
func (v *ValidatorKeysManager) InitKeys() error {
	if v.init {
		return errors.New("keys are already initialized")
	}
	if _, err := os.Stat(v.validatorKeysPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Error checking key file: %w", err)
		}
		Logger.Warn().Msgf("Keys file not found, generating new keys at path: %s", v.validatorKeysPath)
		if err := v.generateKeys(); err != nil {
			return fmt.Errorf("Error generating keys: %w", err)
		}
		if err := v.dumpKeys(); err != nil {
			return fmt.Errorf("Error saving keys: %w", err)
		}
		v.init = true
		return nil
	}
	if err := v.loadKeys(); err != nil {
		return fmt.Errorf("Error loading keys: %w", err)
	}
	v.init = true
	return nil
}

func (v *ValidatorKeysManager) GetKey(shardId types.ShardId) (*ecdsa.PrivateKey, error) {
	if !v.init {
		return nil, errors.New("keys are not initialized")
	}
	if uint32(shardId) >= v.nShards {
		return nil, errors.New("shardId is out of range")
	}
	return v.keys[shardId], nil
}

func (v *ValidatorKeysManager) GetPublicKey(shardId types.ShardId) ([]byte, error) {
	if !v.init {
		return nil, errors.New("keys are not initialized")
	}
	if uint32(shardId) >= v.nShards {
		return nil, errors.New("shardId is out of range")
	}
	return gethcrypto.CompressPubkey(&v.keys[shardId].PublicKey), nil
}

func (v *ValidatorKeysManager) GetKeys() ([]*ecdsa.PrivateKey, error) {
	if !v.init {
		return nil, errors.New("keys are not initialized")
	}
	return v.keys, nil
}
