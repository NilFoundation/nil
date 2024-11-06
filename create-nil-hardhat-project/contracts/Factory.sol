// SPDX-License-Identifier: MIT
pragma solidity ^0.8.11;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract Factory {
    using Nil for address;

    // Mapping to store contract addresses by name
    mapping(string => address) public contractsByName;

    // Event to emit when a contract is deployed
    event ContractDeployed(string indexed name, address indexed deployedAddress);

    function deploy(
        string memory name,
        uint shardId,
        bytes memory code,
        uint256 salt
    ) public {
        // Deploy the contract
        address contractAddress = Nil.asyncDeploy(shardId, msg.sender, 0, code, salt);

        // Store the deployed contract address by name
        contractsByName[name] = contractAddress;

        // Emit the event
        emit ContractDeployed(name, contractAddress);
    }

    // Getter function to retrieve contract address by name
    function getContractAddress(string memory name) public view returns (address) {
        return contractsByName[name];
    }
}
