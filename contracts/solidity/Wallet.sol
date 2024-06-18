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

    function asyncCall(address dst, address refundTo, uint gas, bool deploy, uint value, bytes calldata call_data) onlyExternal public {
        nil.async_call(dst, refundTo, gas, deploy, value, call_data);
    }

    function syncCall(address dst, uint gas, uint value, bytes memory call_data) onlyExternal public {
        (bool success, bytes memory returnData) = dst.call{value: value, gas: gas}(call_data);
        require(success, "Call failed");
    }

    function verifyExternal(uint256 hash, bytes memory signature) external view returns (bool) {
        return nil.validateSignature(pubkey, hash, signature);
    }
}
