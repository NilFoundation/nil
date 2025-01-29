package internal

import (
	"crypto/rand"
	"errors"
	"os"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

//////////////////////// Types ////////////////////////

type dumpedP2pKeys struct {
	PrivateKey hexutil.Bytes `yaml:"privateKey"`
	PublicKey  hexutil.Bytes `yaml:"publicKey"`
	Identity   string        `yaml:"identity"`
}

type KeyManager interface {
	GenerateKey() (Key, error)
	LoadKey([]byte) (Key, error)
}

type Key interface {
	Dump() ([]byte, error)
}

//////////////////////// Libp2pKeyManager ////////////////////////

type Libp2pKeyManager struct{}

type Libp2pKey struct {
	crypto.PrivKey
}

func (k Libp2pKey) SerializeKeys() ([]byte, []byte, peer.ID, error) {
	privKey := k.PrivKey
	privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, nil, "", err
	}

	pubKey := privKey.GetPublic()
	pubKeyBytes, err := crypto.MarshalPublicKey(pubKey)
	if err != nil {
		return nil, nil, "", err
	}

	identity, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, nil, "", err
	}

	return privKeyBytes, pubKeyBytes, identity, nil
}

func (k Libp2pKey) Dump() ([]byte, error) {
	privKeyBytes, pubKeyBytes, identity, err := k.SerializeKeys()
	if err != nil {
		return nil, err
	}

	dumpedKeys := &dumpedP2pKeys{
		PrivateKey: privKeyBytes,
		PublicKey:  pubKeyBytes,
		Identity:   identity.String(),
	}

	data, err := yaml.Marshal(dumpedKeys)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (Libp2pKeyManager) GenerateKey() (Key, error) {
	res, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	return Libp2pKey{PrivKey: res}, err
}

func (Libp2pKeyManager) LoadKey(data []byte) (Key, error) {
	dumpedKeys := &dumpedP2pKeys{}

	if err := yaml.Unmarshal(data, dumpedKeys); err != nil {
		return nil, err
	}

	privKey, err := crypto.UnmarshalPrivateKey(dumpedKeys.PrivateKey)
	if err != nil {
		return nil, err
	}

	pubKey, err := crypto.UnmarshalPublicKey(dumpedKeys.PublicKey)
	if err != nil {
		return nil, err
	}

	id, err := peer.Decode(dumpedKeys.Identity)
	if err != nil {
		return nil, err
	}
	identity, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, err
	}
	if id != identity {
		return nil, errors.New("identity mismatch")
	}

	if !privKey.GetPublic().Equals(pubKey) {
		return nil, errors.New("public key mismatch")
	}

	return Libp2pKey{PrivKey: privKey}, nil
}

//////////////////////// Helper functions ////////////////////////

func LoadKey(fileName string, km KeyManager) (Key, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return km.LoadKey(data)
}

const filePermissions = 0o644

func DumpKey(fileName string, key Key) error {
	data, err := key.Dump()
	if err != nil {
		return err
	}
	return os.WriteFile(fileName, data, filePermissions)
}

func LoadOrGenerateKey(fileName string, km KeyManager) (Key, error) {
	_, err := os.Stat(fileName)
	if err != nil {
		if !os.IsNotExist(err) {
			Logger.Error().Err(err).Msg("Error checking key file")
			return nil, err
		}

		key, err := km.GenerateKey()
		if err != nil {
			Logger.Error().Err(err).Msg("Error generating key")
			return nil, err
		}
		if err := DumpKey(fileName, key); err != nil {
			Logger.Error().Err(err).Msg("Error saving key")
			return nil, err
		}
	}

	return LoadKey(fileName, km)
}
