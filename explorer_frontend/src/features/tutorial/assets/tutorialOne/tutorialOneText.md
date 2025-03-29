# Async calls and default tokens

On =nil;, contracts deployed on different shards can [**use async calls**](https://docs.nil.foundation/nil/smart-contracts/handling-async-execution) to communicate with each other.

This is typically done importing the *Nil.sol* contract and using the *Nil.asyncCall()* function:

```solidity
function asyncCall(
    address dst,
    address bounceTo,
    uint value,
    bytes memory callData
) {}
```

Since *Nil.asyncCall()* includes the *value* argument, this function can be used to [**pass default tokens**](https://docs.nil.foundation/nil/smart-contracts/tokens) between contracts on different shards.

## Task

This tutorial includes two contracts:

* *Caller*
* *Receiver*

The goal of *Caller* is to send *300_000* default tokens to *Receiver* by invoking the *sendValue()* function.

The goal of *Receiver* is to receive tokens sent from *Caller*.

To complete this tutorial:

* Complete the *Caller* contract so that *sendValue()* sends funds to *Receiver* by calling the *deposit()* function.
* Complete the *Receiver* contract so that it can receive tokens via the *deposit()* function.

## Checks

This tutorial is verified once the following checks are passed:

* *Caller* and *Receiver* are compiled and deployed.
* *sendValue()* is successfully called inside *Caller*.
* *Receiver* receives tokens from *Caller*.

To run these checks:

1. Compile both contracts
2. Click on 'Run Checks'