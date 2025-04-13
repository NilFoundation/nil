package keys

import (
	"errors"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/crypto/bls"
	"github.com/NilFoundation/nil/nil/internal/crypto/bls/kyber"
	"gopkg.in/yaml.v3"
)

var Logger = logging.NewLogger("keys")

var errKeysNotInitialized = errors.New("keys are not initialized")

type dumpedValidatorKey struct {
	PrivateKey hexutil.Bytes `yaml:"privateKey"`
	PublicKey  hexutil.Bytes `yaml:"publicKey"`
}

type ValidatorKeysManager struct {
	validatorKeyPath string
	key              bls.PrivateKey
}

func NewValidatorKeyManager(validatorKeyPath string) *ValidatorKeysManager {
	return &ValidatorKeysManager{
		validatorKeyPath: validatorKeyPath,
	}
}

func (v *ValidatorKeysManager) generateKey() {
	v.key = kyber.NewRandomKey()
}

const filePermissions = 0o644

func (v *ValidatorKeysManager) dumpKey() error {
	sk, err := v.key.Marshal()
	if err != nil {
		return err
	}

	pk, err := v.key.PublicKey().Marshal()
	if err != nil {
		return err
	}

	dumpedKey := &dumpedValidatorKey{
		PrivateKey: sk,
		PublicKey:  pk,
	}

	data, err := yaml.Marshal(dumpedKey)
	if err != nil {
		return err
	}
	return os.WriteFile(v.validatorKeyPath, data, filePermissions)
}

func (v *ValidatorKeysManager) loadKey() error {
	Logger.Info().Msgf("Loading key from path: %s", v.validatorKeyPath)
	data, err := os.ReadFile(v.validatorKeyPath)
	if err != nil {
		return err
	}

	dumpedKey := &dumpedValidatorKey{}
	if err := yaml.Unmarshal(data, dumpedKey); err != nil {
		return err
	}

	privKey, err := kyber.PrivateKeyFromBytes(dumpedKey.PrivateKey)
	if err != nil {
		return err
	}
	pubKey, err := kyber.PublicKeyFromBytes(dumpedKey.PublicKey)
	if err != nil {
		return err
	}
	if !pubKey.Equal(privKey.PublicKey()) {
		return errors.New("public key mismatch")
	}
	v.key = privKey
	return nil
}

// This functions initializes key by loading it from the file if it exists,
// or generating new key and saving it to the file.
func (v *ValidatorKeysManager) InitKey() error {
	if v.key != nil {
		return errors.New("key is already initialized")
	}
	if _, err := os.Stat(v.validatorKeyPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("error checking key file: %w", err)
		}
		Logger.Warn().Msgf("Key file not found, generating new key at path: %s", v.validatorKeyPath)
		v.generateKey()
		if err := v.dumpKey(); err != nil {
			return fmt.Errorf("error saving key: %w", err)
		}
		return nil
	}
	if err := v.loadKey(); err != nil {
		return fmt.Errorf("error loading key: %w", err)
	}
	return nil
}

func (v *ValidatorKeysManager) GetKey() (bls.PrivateKey, error) {
	if v.key == nil {
		return nil, errKeysNotInitialized
	}
	return v.key, nil
}

func (v *ValidatorKeysManager) GetPublicKey() ([]byte, error) {
	if v.key == nil {
		return nil, errKeysNotInitialized
	}
	return v.key.PublicKey().Marshal()
}

func (v *ValidatorKeysManager) GetKeysPath() string {
	return v.validatorKeyPath
}
