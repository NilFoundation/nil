package commands

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type CommandDefinition struct {
	ExecCommands   []*exec.Cmd
	ExpectedResult types.TaskOutputArtifacts
}

type CommandConfig struct {
	NilRpcEndpoint      string
	ProofProducerBinary string
	OutDir              string
}

type Command interface {
	MakeCommandDefinition(task *types.Task) (*CommandDefinition, error)
}

type BeforeCommandExecuted interface {
	BeforeCommandExecuted(ctx context.Context, task *types.Task, results types.TaskOutputArtifacts) error
}

type AfterCommandExecuted interface {
	AfterCommandExecuted(task *types.Task, results types.TaskOutputArtifacts) (types.TaskResultData, error)
}

type cmdCommon struct {
	binary string
	outDir string
}

func makeCmdCommon(config CommandConfig) cmdCommon {
	return cmdCommon{binary: config.ProofProducerBinary, outDir: config.OutDir}
}

func circuitTypeToArg(ct types.CircuitType) string {
	switch ct {
	case types.None:
		return "none"
	case types.CircuitBytecode:
		return "bytecode"
	case types.CircuitReadWrite:
		return "rw"
	case types.CircuitZKEVM:
		return "zkevm"
	case types.CircuitCopy:
		return "copy"
	default:
		panic("Unknown circuit type")
	}
}

func circuitIdx(ct types.CircuitType) uint8 {
	return uint8(ct)
}

func aggregateCircuitDependencies(
	task *types.Task,
	dependencyType types.TaskType,
	resultType types.ProverResultType,
) ([]string, error) {
	depFiles := make(map[types.CircuitType]string)
	for _, res := range task.DependencyResults {
		if res.TaskType == dependencyType {
			path, ok := res.OutputArtifacts[resultType]
			if !ok {
				return nil,
					fmt.Errorf("inconsistent task %v , dependencyType %v has no expected result %v",
						task.Id.String(),
						dependencyType.String(),
						resultType.String())
			}
			depFiles[res.CircuitType] = path
		}
	}
	if len(depFiles) != int(types.CircuitAmount) {
		return nil,
			fmt.Errorf("insufficient input for task %v with type %v on %v dependency: expected %d, actual %d",
				task.Id.String(),
				task.TaskType.String(),
				dependencyType,
				types.CircuitAmount,
				len(depFiles))
	}
	// Dependency files must be arranged in the same order for each task
	// We use the order of ascending circuit indices
	arrangedInputs := make([]string, 0)
	for ct := range types.Circuits() {
		arrangedInputs = append(arrangedInputs, depFiles[ct])
	}
	return arrangedInputs, nil
}

func findDependencyResult(
	task *types.Task,
	dependencyType types.TaskType,
	resultType types.ProverResultType,
) (string, error) {
	foundFile := ""
	for _, res := range task.DependencyResults {
		if res.TaskType == dependencyType {
			if foundFile != "" {
				return "", fmt.Errorf(
					"more then one %v files was found as a result for %v dependency",
					resultType.String(), dependencyType.String())
			}
			path, ok := res.OutputArtifacts[resultType]
			if !ok {
				return "", fmt.Errorf(
					"DependencyType %v  has no expected result %v",
					dependencyType.String(), resultType.String())
			}
			foundFile = path
		}
	}
	return foundFile, nil
}
