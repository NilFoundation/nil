package minter

const (
	withdrawFlag = "withdraw"
)

var params = &minterParams{}

type minterParams struct {
	withdraw bool
}
