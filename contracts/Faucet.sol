// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./nil.sol";
import "./Wallet.sol";

contract Faucet {
    event Deploy(address addr);
    event Send(address addr, uint256 value);

    function verifyExternal(uint256, bytes memory) external view returns (bool) {
        return true;
    }

    function withdrawTo(address payable addr, uint256 value) public {
        addr.send(value);
    }

    function createWallet(bytes memory owner_pubkey, bytes32 salt, uint256 value) external returns (address) {
        Wallet wallet = new Wallet{salt: salt}(owner_pubkey);
        address addr = address(wallet);
        emit Deploy(addr);

        bool success = payable(addr).send(value);
        require(success, "Send value failed");
        emit Send(addr, value);

        return addr;
    }

    function send(bytes calldata message) public payable {
        nil.send_msg(gasleft(), message);
    }
}
