package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	socketDir      = "nil_block_generator_tmp"
	configFilePath = "config.json"
	nilDBPath      = "test.db"
	fileMode       = 0o666
	directoryMode  = 0o744
)

type Contract struct {
	Path    string `json:"path"`
	Address string `json:"address"`
}

func NewContract(path, adr string) *Contract {
	return &Contract{
		Path:    path,
		Address: adr,
	}
}

type Call struct {
	ContractName string   `json:"contractName"`
	Method       string   `json:"method"`
	AbiPath      string   `json:"abiPath"`
	Address      string   `json:"address"`
	Args         []string `json:"args"`
	Count        int      `json:"count"`
}

func NewCall(contractName, method, abiPath, address string, args []string, count int) *Call {
	return &Call{
		ContractName: contractName,
		Method:       method,
		AbiPath:      abiPath,
		Address:      address,
		Args:         args,
		Count:        count,
	}
}

type Config struct {
	HttpUrl         string              `json:"httpUrl"`
	SmartAccountAdr string              `json:"smartAccountAdr"`
	PrivateKey      string              `json:"privatekey"`
	Contracts       map[string]Contract `json:"conracts"`
	Calls           []Call              `json:"calls"`
}

func NewConfig() *Config {
	return &Config{
		HttpUrl:         "",
		SmartAccountAdr: "",
		PrivateKey:      "",
		Contracts:       make(map[string]Contract),
	}
}

func CleanFiles() error {
	err := os.RemoveAll(socketDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = os.Remove(configFilePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = os.RemoveAll(nilDBPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ReadConfigFromFile() (*Config, error) {
	b, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return nil, err
	}
	// cleanup socket
	err = os.Remove(filepath.Join(socketDir, "httpd.sock"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &cfg, nil
}

func WriteConfigToFile(cfg *Config) error {
	jsonCfg, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	err = os.WriteFile(configFilePath, jsonCfg, fileMode)
	return err
}

func InitConfig(httpUrl, adr, hexKey string) error {
	cfg := NewConfig()
	cfg.HttpUrl = httpUrl
	cfg.SmartAccountAdr = adr
	cfg.PrivateKey = hexKey
	return WriteConfigToFile(cfg)
}

func AddContract(name, path, adr string) error {
	cfg, err := ReadConfigFromFile()
	if err != nil {
		return err
	}
	cfg.Contracts[name] = *NewContract(path, adr)
	return WriteConfigToFile(cfg)
}

func AddCall(contractName string, method string, args []string, count int) error {
	cfg, err := ReadConfigFromFile()
	if err != nil {
		return err
	}
	contract, err := cfg.GetContract(contractName)
	if err != nil {
		return err
	}
	abiPath := contract.Path + ".abi"
	cfg.Calls = append(cfg.Calls, *NewCall(contractName, method, abiPath, contract.Address, args, count))
	return WriteConfigToFile(cfg)
}

func ShowContracts(writer io.StringWriter) error {
	cfg, err := ReadConfigFromFile()
	if err != nil {
		return err
	}
	for contractName, contract := range cfg.Contracts {
		_, err = writer.WriteString(fmt.Sprintf("%s: %s: %s\n", contractName, contract.Address, contract.Path))
		if err != nil {
			return err
		}
	}
	return nil
}

func ShowCalls(writer io.StringWriter) error {
	cfg, err := ReadConfigFromFile()
	if err != nil {
		return err
	}
	for _, call := range cfg.Calls {
		_, err = writer.WriteString(fmt.Sprintf("%s: %s, %d\n", call.ContractName, call.Method, call.Count))
		if err != nil {
			return err
		}
	}
	return nil
}

func GetSockPath() (string, error) {
	err := os.Mkdir(socketDir, directoryMode)
	if err != nil {
		return "", err
	}
	httpUrl := "unix://" + filepath.Join(socketDir, "httpd.sock")
	return httpUrl, nil
}

func (cfg *Config) GetContract(contractName string) (*Contract, error) {
	contract, ok := cfg.Contracts[contractName]
	if !ok {
		return nil, fmt.Errorf("contract %s not deployed", contractName)
	}
	return &contract, nil
}
