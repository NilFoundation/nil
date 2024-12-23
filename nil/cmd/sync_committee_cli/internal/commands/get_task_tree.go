package commands

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type GetTaskTreeParams struct {
	ExecutorParams
	TaskId public.TaskId
}

func (p *GetTaskTreeParams) GetExecutorParams() *ExecutorParams {
	return &p.ExecutorParams
}

func GetTaskTree(ctx context.Context, params *GetTaskTreeParams, api public.TaskDebugApi) (CmdOutput, error) {
	taskTree, err := api.GetTaskTree(ctx, params.TaskId)
	if err != nil {
		return EmptyOutput, fmt.Errorf("failed to get task tree from debug API: %w", err)
	}
	if taskTree == nil {
		return EmptyOutput, fmt.Errorf("%w: root task with id=%s is not found", ErrNoDataFound, params.TaskId)
	}

	treeOutput, err := buildTreeOutput(taskTree)
	if err != nil {
		return EmptyOutput, fmt.Errorf("failed build string representation of a tree: %w", err)
	}

	return treeOutput, nil
}

func buildTreeOutput(tree *public.TaskTreeView) (CmdOutput, error) {
	var builder outputBuilder

	var toTreeRec func(tree *public.TaskTreeView, prefix string, isLast bool, currentDepth int) error
	toTreeRec = func(node *public.TaskTreeView, prefix string, isLast bool, currentDepth int) error {
		if currentDepth > public.TreeViewDepthLimit {
			return public.TreeDepthExceededErr(node.Task.Id)
		}

		builder.WriteString(prefix)
		if isLast {
			builder.WriteString("└── ")
			prefix += "    "
		} else {
			builder.WriteString("├── ")
			prefix += "│   "
		}

		var execTimeStr string
		if execTime := node.Task.ExecutionTime; execTime != nil {
			execTimeStr = YellowStr(" (%s)", execTime.String())
		}

		builder.WriteLine(
			node.Task.Id.String(), " ", GreenStr("%s %s", node.Task.Type, node.Task.CircuitType), execTimeStr,
		)

		var statusStr string
		var errorText string
		if node.Result != nil && !node.Result.IsSuccess {
			statusStr = RedStr("%s", node.Task.Status)
			errorText = " " + node.Result.ErrorText
		} else {
			statusStr = CyanStr("%s", node.Task.Status)
		}

		builder.WriteLine(
			prefix, "  Owner=", CyanStr("%s", node.Task.Owner), " Status=", statusStr, errorText,
		)

		if len(node.Dependencies) == 0 {
			return nil
		}

		deps := slices.SortedFunc(maps.Values(node.Dependencies), func(l *public.TaskTreeView, r *public.TaskTreeView) int {
			return strings.Compare(l.Task.Id.String(), r.Task.Id.String())
		})

		for i, dependency := range deps {
			isLastDep := i == len(node.Dependencies)-1
			if err := toTreeRec(dependency, prefix, isLastDep, currentDepth+1); err != nil {
				return err
			}
		}

		return nil
	}

	if err := toTreeRec(tree, "", true, 0); err != nil {
		return "", err
	}
	return builder.String(), nil
}
