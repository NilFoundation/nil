// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { OwnableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import { StorageUtils } from "../../common/libraries/StorageUtils.sol";
import { NilConstants } from "../../common/libraries/NilConstants.sol";
import { PausableUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import { AccessControlEnumerableUpgradeable } from "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import { INilAccessControlUpgradeable } from "../../interfaces/INilAccessControlUpgradeable.sol";

contract MyLogicBasic is
  OwnableUpgradeable,
  PausableUpgradeable,
  ReentrancyGuardUpgradeable,
  AccessControlEnumerableUpgradeable
{
  bytes32 private constant SimpleStorageSlot = 0x6f5d6a4da33e672a8e76a897ec60ff29e7b33e7d5850f7c4b13b8fb496d1063d;

  uint256 public value;

  function initialize(address ownerAddress, address adminAddress, uint256 _value) public initializer {
    // // Initialize the Ownable contract with the owner address
    OwnableUpgradeable.__Ownable_init(ownerAddress);

    // Initialize the Pausable contract
    PausableUpgradeable.__Pausable_init();

    ReentrancyGuardUpgradeable.__ReentrancyGuard_init();

    // Initialize the AccessControlEnumerable contract
    __AccessControlEnumerable_init();

    _setRoleAdmin(NilConstants.OWNER_ROLE, NilConstants.OWNER_ROLE);
    _grantRole(NilConstants.OWNER_ROLE, ownerAddress);

    _setSimpleStorageValue(123);

    value = _value;
  }

  function hasOwnerRole(address user) public view returns (bool) {
    return hasRole(NilConstants.OWNER_ROLE, user);
  }

  function getAllAdmins() external view returns (address[] memory) {
    return getRoleMembers(DEFAULT_ADMIN_ROLE);
  }

  function getAllOwners() external view returns (address[] memory) {
    return getRoleMembers(NilConstants.OWNER_ROLE);
  }

  function setValue(uint256 _value) public {
    value = _value;
  }

  function getValue() public view returns (uint256) {
    return value;
  }

  function getImplementation() public view returns (address) {
    return StorageUtils.getImplementationAddress(NilConstants.IMPLEMENTATION_SLOT);
  }

  function _setSimpleStorageValue(uint256 _value) internal {
    assembly {
      sstore(SimpleStorageSlot, _value)
    }
  }

  function getSimpleStorageValue() public view returns (uint256) {
    return _getSimpleStorageValue();
  }

  function _getSimpleStorageValue() internal view returns (uint256 slotValue) {
    assembly {
      slotValue := sload(SimpleStorageSlot)
    }
  }
}
