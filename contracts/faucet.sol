// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Faucet {
    function withdrawTo(address payable addr, uint256 value) public {
        addr.send(value);
    }
}
