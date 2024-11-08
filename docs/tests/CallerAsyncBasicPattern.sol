// SPDX-License-Identifier: MIT

//startCallerAsyncBasicPattern
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Caller {
    using Nil for address;

    event CallCompleted(address indexed dst);

    function call(address dst) public payable {
        dst.asyncCall(address(0), 0, abi.encodeWithSignature("funcName"));
        emit CallCompleted(dst);
    }
}
//endCallerAsyncBasicPattern
