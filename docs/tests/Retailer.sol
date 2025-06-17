// SPDX-License-Identifier: MIT

//startRetailerContract
pragma solidity ^0.8.0;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Retailer is NilBase {
    using Nil for address;

    receive() external payable {}

    function orderProduct(address dst, string calldata name) public async(2_000_000) {
        dst.asyncCall(
            Nil.msgSender(),
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
