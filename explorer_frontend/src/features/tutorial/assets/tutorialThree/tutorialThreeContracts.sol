// SPDX-License-Identifier: MIT

pragma solidity ^0.8.21;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Requester is NilBase {
  using Nil for address;
  uint256 private num1 = 5;
  uint256 private num2 = 10;
  boolean private result;

  function requestMultiplication(address dst) public payable {
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
  }

  function getResult() public view returns (bool) {
    return result;
  }
}

contract RequestedContract {
  function multiply(uint256 num1, uint256 num2) public pure returns (uint256) {
    return num1 * num2;
  }
}