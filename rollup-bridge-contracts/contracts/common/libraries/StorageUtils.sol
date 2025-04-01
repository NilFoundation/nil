// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

library StorageUtils {
  struct AddressSlot {
    address value;
  }

  /**
   * @dev Returns the current implementation address stored at the given slot.
   */
  function getImplementationAddress(bytes32 slot) internal view returns (address) {
    return getAddressSlot(slot).value;
  }

  /**
   * @dev Returns an `AddressSlot` with member `value` located at `slot`.
   */
  function getAddressSlot(bytes32 slot) internal pure returns (AddressSlot storage r) {
    assembly {
      r.slot := slot
    }
  }
}
