// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";

contract MyTransparentUpgradeableProxy is TransparentUpgradeableProxy {
  constructor(address _logic, address admin_, bytes memory _data) TransparentUpgradeableProxy(_logic, admin_, _data) {
    // Constructor logic can be added here if needed
  }
  /// @notice Fetches the implementation contract address
  function fetchImplementation() external view returns (address) {
    bytes32 IMPLEMENTATION_SLOT = 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;
    address implementation;
    assembly {
      implementation := sload(IMPLEMENTATION_SLOT)
    }
    return implementation;
  }

  function fetchAdmin() external view returns (address) {
    // This is the correct admin slot used by OpenZeppelin's TransparentUpgradeableProxy
    bytes32 ADMIN_SLOT = 0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103;
    address admin;
    assembly {
      admin := sload(ADMIN_SLOT)
    }
    return admin;
  }

  receive() external payable {}
}
