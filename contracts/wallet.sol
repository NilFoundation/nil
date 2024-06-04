// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.9;

import "./nil.sol";

contract Wallet {

    function send(bytes memory message) public payable {
        nil.send_msg(gasleft(), message);
    }
}
