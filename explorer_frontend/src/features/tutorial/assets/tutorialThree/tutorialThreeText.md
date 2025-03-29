# Request/response pattern

[The request/response pattern](https://docs.nil.foundation/nil/smart-contracts/handling-async-execution#examples) works as follows:

* A contract uses the *Nil.sendRequest()* function to call another contract asynchronously
* The contract being called executes a function and sends the result back to the caller
* The caller contract executes another function based on the response

Note that the caller contract does not have to wait for the response from the callee. The caller contract can freely execute other functions or respond to other calls until a response to *Nil.sendRequest()* arrives.

To use *Nil.sendRequest()*:

```solidity
Nil.sendRequest(
  dst,
  0,
  Nil.ASYNC_REQUEST_MIN_GAS,
  context,
  callData
);
```

## Task

This tutorial includes two contracts:

* *Requester*
* *RequestedContract*

*Requester* holds two private numeric values. The contract must do the following:

* Form a valid *context* that contains the product of multiplication of these two values.
* Form valid *callData* calling the *multiply()* function inside *RequestedContract*.
* Use *Nil.sendRequest()* to call *RequestedContract* with the formed *context* and *callData*.
* Upon receiving a response, *Requester* must execute the *checkResult()* function that verifies whether the result received from *RequestedContract* is a valid product of multiplication of the two private numeric values.
* The result of the check must be stored inside the private *result* variable.

*RequestedContract* contains the *multiply()* function that receives two numeric values and returns the result.

To complete this tutorial:

* Complete both contracts so that *Requester* sends a valid request to *RequestedContract*.
* On receiving a response, *Requester* must return a boolean value signifying whether the result received from *RequestedContract* is a valid multiplication product.

## Checks

This tutorial is verified once the following checks are passed:

* *Requester* and *RequestedContract* are compiled and deployed.
* *Requester* successfully uses *Nil.sendRequest()* to call *RequestedContract*
* *RequestedContract* multiplies the provided values and returns the result
* *Requester* receives the result and verifies it

To run these checks:

1. Compile both contracts
2. Click on 'Run Checks'