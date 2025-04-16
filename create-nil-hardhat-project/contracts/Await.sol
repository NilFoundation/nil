// SPDX-License-Identifier: MIT
pragma solidity ^0.8.11;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Await {
    using Nil for address;

    uint256 public result;

    function callback(
        bool success,
        bytes memory returnData,
        bytes memory
    ) public {
        require(success == true, "Result not true");
        result = abi.decode(returnData, (uint256));
    }

    function call(address dst) public{
        bytes memory context = abi.encodeWithSelector(this.callback.selector);
        Nil.sendRequest(
            dst,
            0,
            Nil.ASYNC_REQUEST_MIN_GAS,
            context,
            abi.encodeWithSignature("getValue()")
        );
    }
}
