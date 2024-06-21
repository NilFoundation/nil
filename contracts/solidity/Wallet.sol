// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "./Nil.sol";

contract Wallet is NilBase {

    bytes pubkey;

    receive() external payable {}

    constructor(bytes memory _pubkey) {
        pubkey = _pubkey;
    }

    function send(bytes calldata message) onlyExternal public {
        Nil.sendMessage(gasleft(), message);
    }

    function asyncCall(address dst, address refundTo, uint gas, bool deploy, uint value, bytes calldata callData) onlyExternal public {
        Nil.asyncCall(dst, refundTo, gas, deploy, value, callData);
    }

    function syncCall(address dst, uint gas, uint value, bytes memory call_data) onlyExternal public {
        (bool success,) = dst.call{value: value, gas: gas}(call_data);
        require(success, "Call failed");
    }

    function verifyExternal(uint256 hash, bytes calldata signature) external view returns (bool) {
        return Nil.validateSignature(pubkey, hash, signature);
    }
}
