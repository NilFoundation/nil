// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract AccessRestriction {
    address public owner;
    mapping(address => bool) public admins;

    uint public controlValue = 0;

    constructor() {
        owner = msg.sender;
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "Access denied: Only owner");
        _;
    }

    modifier onlyAdmin() {
        require(admins[msg.sender], "Access denied: Only admin");
        _;
    }

    function addAdmin(address admin) external onlyOwner {
        require(admin != address(0), "Invalid address");
        admins[admin] = true;
    }

    function removeAdmin(address admin) external onlyOwner {
        require(admins[admin], "Address is not an admin");
        admins[admin] = false;
    }

    function accessRestrictionAction() external onlyAdmin {
        controlValue += 1;
    }
}