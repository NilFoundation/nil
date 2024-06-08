// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./nil.sol";

contract Faucet {
    // This function signature is temporary.
    // Eventually, contracts are going to store pubkey(s) in their storage,
    // and verifyExternal is going to receive the actual message in full.
    function verifyExternal(bytes memory pubkey, uint256 hash, bytes memory signature) public view returns (bool) {
        return nil.validateSignature(pubkey, hash, signature);
    }

    function withdrawTo(address payable addr, uint256 value) public {
        addr.send(value);
    }
}
