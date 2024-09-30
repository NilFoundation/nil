package internal

import (
	"os"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

type dumpedKeys struct {
	PrivateKey hexutil.Bytes `yaml:"privateKey"`
	PublicKey  hexutil.Bytes `yaml:"publicKey"`
	Identity   string        `yaml:"identity"`
}

func SerializeKeys(privKey crypto.PrivKey) ([]byte, []byte, peer.ID, error) {
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

func DumpKeys(privKey crypto.PrivKey, fileName string) error {
	privKeyBytes, pubKeyBytes, identity, err := SerializeKeys(privKey)
	if err != nil {
		return err
	}

	dumpedKeys := &dumpedKeys{
		PrivateKey: privKeyBytes,
		PublicKey:  pubKeyBytes,
		Identity:   identity.String(),
	}

	data, err := yaml.Marshal(dumpedKeys)
	if err != nil {
		return err
	}

	return os.WriteFile(fileName, data, 0o600)
}

func LoadKeys(fileName string) (crypto.PrivKey, crypto.PubKey, peer.ID, error) {
	dumpedKeys := &dumpedKeys{}

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, nil, "", err
	}
	if err := yaml.Unmarshal(data, dumpedKeys); err != nil {
		return nil, nil, "", err
	}

	privKey, err := crypto.UnmarshalPrivateKey(dumpedKeys.PrivateKey)
	if err != nil {
		return nil, nil, "", err
	}

	pubKey, err := crypto.UnmarshalPublicKey(dumpedKeys.PublicKey)
	if err != nil {
		return nil, nil, "", err
	}

	id, err := peer.Decode(dumpedKeys.Identity)
	if err != nil {
		return nil, nil, "", err
	}

	return privKey, pubKey, id, nil
}
