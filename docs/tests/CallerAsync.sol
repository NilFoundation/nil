// SPDX-License-Identifier: MIT

//startCallerAsync
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract CallerAsync {
    using Nil for address;

    event CallCompleted(address indexed dst);

    function call(address dst) public payable {
        dst.asyncCall(
            address(0),
            msg.value,
            abi.encodeWithSignature("funcName")
        );
        emit CallCompleted(dst);
    }
}
//endCallerAsync
