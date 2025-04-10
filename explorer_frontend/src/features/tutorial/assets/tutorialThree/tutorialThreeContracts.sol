// SPDX-License-Identifier: MIT

pragma solidity ^0.8.21;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

/**
 * @title Requester
 * @author =nil; Foundation
 * @notice A contract for requesting the multiplication of two numbers.
 */
contract Requester is NilBase {
  using Nil for address;
  uint256 private num1 = 5;
  uint256 private num2 = 10;
  boolean private result;

  function requestMultiplication(address dst) public payable {
    // TODO: create valid context and callData
    bytes memory context = ;
    bytes memory callData = ;

    Nil.sendRequest(
      dst,
      0,
      Nil.ASYNC_REQUEST_MIN_GAS,
      context,
      callData
    );
  }

  function checkResult(bool success, bytes memory returnData, bytes memory context) 
    public 
    payable
    onlyResponse
    {
    require(success, "Request failed!");
    // TODO: complete the function
  }

  function getResult() public view returns (bool) {
    return result;
  }
}

/**
 * @title RequestedContract
 * @author =nil; Foundation
 * @notice A contract for multiplying two numbers and returning the result.
 */
contract RequestedContract {
  function multiply(uint256 num1, uint256 num2) public pure returns (uint256) {
    return num1 * num2;
  }
}