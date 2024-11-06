// SPDX-License-Identifier: MIT

//startRetailerContract
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Retailer {
    using Nil for address;

    receive() external payable {}

    function orderProduct(address dst, string calldata name) public {
        dst.asyncCall(
            msg.sender,
            msg.sender,
            1_000_000,
            Nil.FORWARD_VALUE,
            0,
            abi.encodeWithSignature("createProduct(string)", name)
        );
    }

    function verifyExternal(
        uint256 hash,
        bytes memory _authData
    ) external view returns (bool) {
        return true;
    }
}

//endRetailerContract