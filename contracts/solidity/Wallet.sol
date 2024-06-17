// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "./nil.sol";

contract Wallet is NilBase {

    bytes pubkey;

    receive() external payable {}

    constructor(bytes memory _pubkey) {
        pubkey = _pubkey;
    }

    function send(bytes calldata message) onlyExternal public {
        nil.send_msg(gasleft(), message);
    }

    function verifyExternal(uint256 hash, bytes memory signature) external view returns (bool) {
        return nil.validateSignature(pubkey, hash, signature);
    }
}
