// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "./nil.sol";

contract Wallet {

    bytes pubkey;

    receive() external payable {}

    constructor(bytes memory _pubkey) {
        pubkey = _pubkey;
    }

    function send(bytes calldata message) public payable {
        nil.send_msg(gasleft(), message);
    }

    function verifyExternal(uint256 hash, bytes memory signature) external view returns (bool) {
        return nil.validateSignature(pubkey, hash, signature);
    }
}
