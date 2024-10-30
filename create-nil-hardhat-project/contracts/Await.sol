// SPDX-License-Identifier: MIT
pragma solidity ^0.8.11;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Await {
    using Nil for address;

    uint256 public result;

    function call(address dst) public{
        bytes memory temp;
        bool ok;
        (temp, ok) = Nil.awaitCall(
            dst,
            Nil.ASYNC_REQUEST_MIN_GAS,
            abi.encodeWithSignature("getValue()")
        );

        require(ok == true, "Result not true");

        result = abi.decode(temp, (uint256));
    }
}
