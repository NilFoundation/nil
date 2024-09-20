// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "./Counter.sol";

contract Requester {
  int32 public value;
  int32 public counterValue;
  uint public intValue;
  string public strValue;

  function get() public view returns(int32) {
      return counterValue;
  }
  
  function responseCounterGet(bool success, bytes memory returnData, bytes memory context) public {
        require(success, "Request failed");
        (intValue, strValue) = abi.decode(context, (uint, string));
        counterValue = abi.decode(returnData, (int32));
  }

  function requestCounterGet(address counter, uint intContext, string memory strContext) public {
        bytes memory context = abi.encodeWithSelector(this.responseCounterGet.selector, intContext, strContext);
        bytes memory callData = abi.encodeWithSignature("getValue()");
        Nil.sendRequest(counter, 0, context, callData);
  }
}