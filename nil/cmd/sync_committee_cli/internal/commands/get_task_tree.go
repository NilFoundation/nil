package commands

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type GetTaskTreeParams struct {
	exec.Params
	TaskId public.TaskId
}

func (p *GetTaskTreeParams) GetExecutorParams() *exec.Params {
	return &p.Params
}

type getTaskTree struct {
	logger logging.Logger
}

func NewGetTaskTreeCmd(logger logging.Logger) *getTaskTree {
	return &getTaskTree{
		logger: logger,
	}
}

func (c *getTaskTree) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()
	cmdParams := &GetTaskTreeParams{
		Params: paramsWithEndpoint.Params,
	}

	cmd := &cobra.Command{
		Use:   "get-task-tree",
		Short: "Retrieve full task tree structure for a specific task",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, cmdParams)
			client := debug.NewTasksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getTaskTree(ctx, cmdParams, client)
			})
		},
	}

	paramsWithEndpoint.bind(cmd)

	const taskIdFlag = "task-id"
	cmd.Flags().Var(&cmdParams.TaskId, taskIdFlag, "root task id")
	if err := cmd.MarkFlagRequired(taskIdFlag); err != nil {
		return nil, err
	}

	return cmd, nil
}

func (c *getTaskTree) getTaskTree(
	ctx context.Context, params *GetTaskTreeParams, api public.TaskDebugApi,
) (exec.CmdOutput, error) {
	taskTree, err := api.GetTaskTree(ctx, params.TaskId)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get task tree from debug API: %w", err)
	}
	if taskTree == nil {
		return exec.EmptyOutput, fmt.Errorf("%w: root task with id=%s is not found", exec.ErrNoDataFound, params.TaskId)
	}

	treeOutput, err := c.buildTreeOutput(taskTree)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed build string representation of a tree: %w", err)
	}

	return treeOutput, nil
}

func (*getTaskTree) buildTreeOutput(tree *public.TaskTreeView) (exec.CmdOutput, error) {
	var builder exec.OutputBuilder

	var toTreeRec func(tree *public.TaskTreeView, prefix string, isLast bool, currentDepth int) error
	toTreeRec = func(node *public.TaskTreeView, prefix string, isLast bool, currentDepth int) error {
		if currentDepth > public.TreeViewDepthLimit {
			return public.TreeDepthExceededErr(node.Id)
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
		if execTime := node.ExecutionTime; execTime != nil {
			execTimeStr = exec.YellowStr(" (%s)", execTime.String())
		}

		builder.WriteLine(
			node.Id.String(), exec.GreenStr(" %s %s", node.Type, node.CircuitType), execTimeStr,
		)

		var statusStr string
		var errorText string
		if node.IsFailed() {
			statusStr = exec.RedStr("%s", node.Status)
			errorText = " " + node.ResultErrorText
		} else {
			statusStr = exec.CyanStr("%s", node.Status)
		}

		builder.WriteLine(
			prefix, "  Owner=", exec.CyanStr("%s", node.Owner), " Status=", statusStr, errorText,
		)

		if len(node.Dependencies) == 0 {
			return nil
		}

		deps := slices.SortedFunc(
			maps.Values(node.Dependencies),
			func(l *public.TaskTreeView, r *public.TaskTreeView) int {
				return strings.Compare(l.Id.String(), r.Id.String())
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
