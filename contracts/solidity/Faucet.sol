// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./Nil.sol";
import "./Wallet.sol";

contract Faucet {
    event Deploy(address addr);
    event Send(address addr, uint256 value);

    function verifyExternal(uint256, bytes memory) external pure returns (bool) {
        return true;
    }

    function withdrawTo(address payable addr, uint256 value) public {
        bool success = addr.send(value);
        require(success, "Send value failed");
        emit Send(addr, value);
    }

    function createWallet(bytes memory ownerPubkey, bytes32 salt, uint256 value) external returns (address) {
        Wallet wallet = new Wallet{salt: salt}(ownerPubkey);
        address addr = address(wallet);
        emit Deploy(addr);

        bool success = payable(addr).send(value);
        require(success, "Send value failed");
        emit Send(addr, value);

        return addr;
    }

    function send(bytes calldata message) public payable {
        Nil.sendMessage(gasleft(), message);
    }
}
