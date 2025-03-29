# Custom tokens in an async call

=nil; allows for [**sending custom tokens in an async call**](https://docs.nil.foundation/nil/smart-contracts/tokens#nft-example).

To achieve this, use the *Nil.asyncCallWithTokens()* function:

```solidity
function asyncCallWithTokens(
    address dst,
    address refundTo,
    address bounceTo,
    uint feeCredit,
    uint8 forwardKind,
    uint value,
    Token[] memory tokens,
    bytes memory callData
) internal {}
```

## Task

This tutorial includes three contracts:

* *Receiver*
* *NFT*

*Receiver* is an empty contract meant to hold the NFT represented by the *NFT* contract.

The *NFT* contract represents a simple non-fungible token. It must be able to handle minting and sending the NFT but only once and the total supply of the NFT can never be greater than one (1).

To complete this tutorial:

* Finish the *NFT* contract so that it can mint one custom token and send it to *Receiver*.

## Checks

This tutorial is verified once the following checks are passed.

* *Receiver* and *NFT* are compiled and deployed.
* *NFT* mints a custom token.
* *NFT* cannot mint tokens after the first mint.
* *NFT* can successfully send the custom token to *Receiver*.

