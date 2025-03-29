# Working with custom tokens

=nil; allows for [**creating custom tokens**](https://docs.nil.foundation/nil/smart-contracts/tokens) and, subsequently, operating on said tokens. 

To achieve this, a smart contract representing a custom token must inherit from *NilCurrencyBase.sol* and, optionally, override some default behaviors.

```solidity
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

contract NewToken is NilTokenBase {}
```

*NilCurrencyBase.sol* contains internal and *onlyExternal* functions for minting, burning and sending a custom token.

## Task

This tutorial includes two contracts:

* *Operator*
* *CustomToken*

*Operator* performs some actions with the custom token that is defined in *CustomToken*. As part of this tutorial, the *Operator* contract must not be modified.

To to complete this tutorial, implement the following features inside the *CustomToken* contract:

* Accept the address of a contract as constructor argument inside the *_operatorAddress* variable.
* Mint a custom token by wrapping the *mintTokenInternal()* function inside the *mintTokenCustom(uint256 amount)* function but only if *mintTokenCustom()* is called from *_operatorAddress*.
* Send a custom token to *_operatorAddress* by wrapping the *sendTokenInternal()* function inside the *sendTokenCustom(uint256 amount)* function.

## Checks

This tutorial is verified once the following checks are passed:

* *Operator* and *CustomToken* are compiled and deployed.
* *mintTokenCustom()* inside *CustomToken* is called successfully and some amount of the custom token is minted.
* *sendTokenCustom()* inside *CustomToken* is called successfully and some amount of the custom token is sent to *Operator*

To run these checks:

1. Compile both contracts
2. Click on 'Run Checks'