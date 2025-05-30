// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

import "../lib/NilTokenBase.sol";

contract Deployer is NilTokenBase {
    address public deployee;

    constructor() payable {}

    function deploy(uint shardId, uint32 _a, uint salt, uint value) public async(1_000_000) {
        bytes memory data = bytes.concat(type(Deployee).creationCode, abi.encode(address(this), _a));
        deployee = Nil.asyncDeploy(shardId, address(this), value, data, salt);
    }

    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }
}

contract Deployee {
    address public deployer;
    uint32 public num;

    constructor(address _deployer, uint32 _num) payable {
        deployer = _deployer;
        num = _num;
    }
}