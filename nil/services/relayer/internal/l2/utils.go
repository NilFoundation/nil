package l2

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func checkIfContractExists(
	ctx context.Context,
	nilClient client.Client,
	addr types.Address,
) (bool, error) {
	accountCode, err := nilClient.GetCode(ctx, addr, "latest")
	if err != nil {
		return false, fmt.Errorf("failed to check presence of the relayer L2 contract (%s): %w", addr, err)
	}
	if len(accountCode) == 0 {
		return false, nil
	}

	return true, nil
}
