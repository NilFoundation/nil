package contracts

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/contracts"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	NameSmartAccount  = "SmartAccount"
	NameFaucet        = "Faucet"
	NameFaucetToken   = "FaucetToken"
	NamePrecompile    = "__Precompile__"
	NameNilTokenBase  = "NilTokenBase"
	NameNilBounceable = "NilBounceable"
	NameNilConfigAbi  = "NilConfigAbi"
	NameL1BlockInfo   = "system/L1BlockInfo"
	NameGovernance    = "system/Governance"
	NameTokenManager  = "NilTokenManager"
	NameRelayer       = "Relayer"
)

var (
	codeCache = concurrent.NewMap[string, types.Code]()
	abiCache  = concurrent.NewMap[string, *abi.ABI]()
)

func GetCode(name string) (types.Code, error) {
	// The result taken from the cache must be cloned.
	if res, ok := codeCache.Get(name); ok {
		return res.Clone(), nil
	}

	code, err := contracts.Fs.ReadFile("compiled/" + name + ".bin")
	if err != nil {
		return nil, err
	}

	res := types.Code(hexutil.FromHex(string(code)))
	codeCache.Put(name, res)
	return res.Clone(), nil
}

func GetAbi(name string) (*abi.ABI, error) {
	if res, ok := abiCache.Get(name); ok {
		return res, nil
	}

	data, err := contracts.Fs.ReadFile("compiled/" + name + ".abi")
	if err != nil {
		return nil, err
	}

	res, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	abiCache.Put(name, &res)
	return &res, nil
}

func GetAbiData(name string) (string, error) {
	data, err := contracts.Fs.ReadFile("compiled/" + name + ".abi")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func CreateDeployPayload(name string, salt []byte, ctorArgs ...any) (types.DeployPayload, error) {
	code, err := GetCode(name)
	if err != nil {
		return types.DeployPayload{}, err
	}

	if len(ctorArgs) != 0 {
		argsPacked, err := NewCallData(name, "", ctorArgs...)
		if err != nil {
			return types.DeployPayload{}, err
		}
		code = append(code, argsPacked...)
	}
	payload := types.BuildDeployPayload(code, common.BytesToHash(salt))

	return payload, nil
}

func CalculateAddress(name string, shardId types.ShardId, salt []byte, ctorArgs ...any) (types.Address, error) {
	payload, err := CreateDeployPayload(name, salt, ctorArgs...)
	if err != nil {
		return types.Address{}, err
	}
	return types.CreateAddress(shardId, payload), nil
}

func NewCallData(fileName, methodName string, args ...any) ([]byte, error) {
	abiCallee, err := GetAbi(fileName)
	if err != nil {
		return nil, err
	}
	return abiCallee.Pack(methodName, args...)
}

func UnpackData(fileName, methodName string, data []byte) ([]any, error) {
	abiCallee, err := GetAbi(fileName)
	if err != nil {
		return nil, err
	}
	return abiCallee.Unpack(methodName, data)
}

func UnpackRelayerResult(data []byte) ([]byte, error) {
	abiRelayer, err := GetAbi(NameRelayer)
	if err != nil {
		return nil, err
	}
	res, err := abiRelayer.Unpack("receiveTx", data)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		return nil, errors.New("invalid relayer result")
	}
	result, ok := res[0].([]byte)
	if !ok {
		return nil, errors.New("invalid relayer result")
	}
	return result, nil
}

type Signature struct {
	Contracts     []string
	FuncName      string
	FuncSignature string
}

var (
	signaturesMap     = map[uint32]*Signature{}
	signaturesMapOnce sync.Once
)

func initSignaturesMap() error {
	if err := initSignaturesMapFromDir("compiled"); err != nil {
		return err
	}
	if err := initSignaturesMapFromDir("compiled/tests"); err != nil {
		return err
	}
	if err := initSignaturesMapFromDir("compiled/system"); err != nil { //nolint:if-return
		return err
	}
	return nil
}

func initSignaturesMapFromDir(dir string) error {
	files, err := contracts.Fs.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".signatures") {
			continue
		}
		file, err := contracts.Fs.Open(dir + "/" + f.Name())
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", f.Name(), err)
		}
		dirContract := strings.TrimPrefix(dir, "compiled")
		dirContract = strings.TrimPrefix(dirContract, "/")
		contractName := strings.TrimSuffix(f.Name(), ".signatures")
		if dirContract != "" {
			contractName = dirContract + "/" + strings.TrimSuffix(f.Name(), ".signatures")
		}

		scanner := bufio.NewScanner(file)
		readingFuncs := false
		for scanner.Scan() {
			line := scanner.Text()
			if readingFuncs {
				if line == "" {
					break
				}
				parts := strings.SplitN(line, ":", 2)
				idStr := strings.TrimSpace(parts[0])
				id, err := strconv.ParseUint(idStr, 16, 32)
				if err != nil {
					return fmt.Errorf("failed to parse func id %s: %w", idStr, err)
				}

				if sig, ok := signaturesMap[uint32(id)]; ok {
					sig.Contracts = append(sig.Contracts, contractName)
				} else {
					signature := &Signature{}
					signature.FuncSignature = strings.TrimSpace(parts[1])
					parts = strings.SplitN(signature.FuncSignature, "(", 2)
					signature.FuncName = strings.TrimSpace(parts[0])
					signature.Contracts = append(signature.Contracts, contractName)
					signaturesMap[uint32(id)] = signature
				}
			} else if line == "Function signatures:" {
				readingFuncs = true
			}
		}
	}
	return nil
}

func GetFuncIdSignatureFromBytes(data []byte) (*Signature, error) {
	if len(data) < 4 {
		return nil, errors.New("data too short")
	}
	return GetFuncIdSignature(binary.BigEndian.Uint32(data[:4]))
}

func GetFuncIdSignature(id uint32) (*Signature, error) {
	signaturesMapOnce.Do(func() {
		check.PanicIfErr(initSignaturesMap())
	})
	if sig, ok := signaturesMap[id]; ok {
		return sig, nil
	}
	return nil, fmt.Errorf("signature not found for id %x", id)
}

func DecodeCallData(method *abi.Method, calldata []byte) (string, string, error) {
	if len(calldata) == 0 {
		return "", "", errors.New("empty calldata")
	}
	if len(calldata) < 4 {
		return "", "", fmt.Errorf("too short calldata: %d", len(calldata))
	}

	if method == nil {
		sig, err := GetFuncIdSignatureFromBytes(calldata)
		if err != nil {
			return "", "", err
		}
		abiContract, err := GetAbi(sig.Contracts[0])
		if err != nil {
			return "", "", fmt.Errorf("failed to get abi: %w", err)
		}
		m, ok := abiContract.Methods[sig.FuncName]
		if !ok {
			return "", "", fmt.Errorf("method not found: %s", sig.FuncName)
		}
		method = &m
	}

	args, err := method.Inputs.Unpack(calldata[4:])
	if err != nil {
		// We found method, but failed to unpack arguments. The user should be warned about it.
		return fmt.Sprintf("%s: failed to unpack arguments: %s", method.Name, err), "", nil
	}
	var relayerData string
	if method.Name == "receiveTx" {
		data, ok := args[5].([]byte)
		if !ok {
			return "", "", fmt.Errorf("failed to cast relayer data: %v", args[5])
		}
		relayerData, _, err = DecodeCallData(nil, data)
		if err != nil {
			return "", "", fmt.Errorf("failed to decode relayer data: %w", err)
		}
	}
	res := method.Name + "("
	adjustArg := func(arg any) string {
		switch v := arg.(type) {
		case []byte:
			return hexutil.Encode(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	for i, arg := range args {
		if i > 0 {
			res += fmt.Sprintf(", %v", adjustArg(arg))
		} else {
			res += adjustArg(arg)
		}
	}
	res += ")"

	return res, relayerData, nil
}
