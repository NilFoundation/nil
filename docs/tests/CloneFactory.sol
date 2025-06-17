// SPDX-License-Identifier: MIT

//startContract
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract MasterChild {
    uint256 private value;

    event ValueChanged(uint256 newValue);

    receive() external payable {}

    function increment() public {
        value += 1;
        emit ValueChanged(value);
    }

    function getValue() public view returns (uint256) {
        return value;
    }
}

contract CloneFactory is NilBase {
    address public masterChildAddress;

    event counterCloneCreated(address indexed addr);

    constructor(address _masterChildAddress) payable {
        masterChildAddress = _masterChildAddress;
    }

    function createCloneBytecode(
        address target
    ) internal returns (bytes memory finalBytecode) {
        bytes memory code = new bytes(55);
        bytes20 targetBytes = bytes20(target);
        assembly {
            let codePtr := add(code, 32)
            mstore(
                codePtr,
                0x3d602d80600a3d3981f3363d3d373d3d3d363d73000000000000000000000000
            )
            mstore(add(codePtr, 0x14), targetBytes)
            mstore(
                add(codePtr, 0x28),
                0x5af43d82803e903d91602b57fd5bf30000000000000000000000000000000000
            )
        }
        finalBytecode = code;
    }

    function createCounterClone(uint256 salt) public async(2_000_000) returns (address) {
        //console.log("createCounterClone 1");
        bytes memory cloneBytecode = createCloneBytecode(masterChildAddress);
        //console.log("createCounterClone 2: %_", cloneBytecode.length);
        uint shardId = Nil.getShardId(masterChildAddress);
        //console.log("createCounterClone 3: %_", shardId);
        uint shardIdFactory = Nil.getShardId(address(this));
        require(
            shardId == shardIdFactory,
            "factory and child are not on the same shard!"
        );
        //console.log("createCounterClone 4");
        address result = Nil.asyncDeploy(
            shardId,
            address(this),
            address(this),
            0,
            Nil.FORWARD_REMAINING,
            0,
            cloneBytecode,
            salt
        );
        emit counterCloneCreated(result);
        //console.log("createCounterClone 5");

        return result;
    }
}

contract FactoryManager is NilBase {
    mapping(uint => address) public factories;
    mapping(uint => address) public masterChildren;
    bytes private code = type(CloneFactory).creationCode;

    event factoryDeployed(address indexed addr);
    event masterChildDeployed(address indexed addr);

    constructor() payable {}

    function deployNewMasterChild(uint shardId, uint256 salt) public async(2_000_000) {
        //console.log("deployNewMasterChild 1");

        address result = Nil.asyncDeploy(
            shardId,
            address(this),
            address(this),
            0,
            Nil.FORWARD_REMAINING,
            0,
            type(MasterChild).creationCode,
            salt
        );
        emit masterChildDeployed(result);

        masterChildren[shardId] = result;
    }

    function deployNewFactory(uint shardId, uint256 salt) public async(2_000_000) {
        require(factories[shardId] == address(0), "factory already exists!");
        bytes memory data = bytes.concat(
            type(CloneFactory).creationCode,
            abi.encode(masterChildren[shardId])
        );
        address result = Nil.asyncDeploy(
            shardId,
            address(this),
            address(this),
            0,
            Nil.FORWARD_REMAINING,
            5000000 * tx.gasprice,
            data,
            salt
        );
        emit factoryDeployed(result);

        factories[shardId] = result;
    }
}

//endContract
