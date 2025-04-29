package feeupdater

import (
	"context"
)

type NilGasPriceOracleContract interface {
	SetOracleFee(ctx context.Context, params feeParams) error
}
