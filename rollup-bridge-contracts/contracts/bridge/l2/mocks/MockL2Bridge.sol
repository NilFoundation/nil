// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IERC165 } from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import { IL2Bridge } from "../interfaces/IL2Bridge.sol";

contract MockL2Bridge is IL2Bridge, IERC165 {
  constructor() {}

  function setRouter(address routerAddress) external {}

  function setMessenger(address messengerAddress) external {}

  function setCounterpartyBridge(address counterpartyBridgeAddress) external {}

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice The address of L1BridgeRouter/L2BridgeRouter contract.
  function router() external view returns (address) {
    return address(0);
  }

  /// @notice The address of Bridge contract on other side (for L1Bridge it would be the bridge-address on L2 and for
  /// L2Bridge this would be the bridge-address on L1)
  function counterpartyBridge() external view returns (address) {
    return address(0);
  }

  /// @notice The address of corresponding L1NilMessenger/L2NilMessenger contract.
  function messenger() external view returns (address) {}

  function setPause(bool _status) external {}

  function transferOwnershipRole(address newOwner) external {}

  function getImplementation() external view returns (address) {
    return address(this);
  }

  /// @inheritdoc IERC165
  function supportsInterface(bytes4 interfaceId) public view override(IERC165) returns (bool) {
    return interfaceId == type(IL2Bridge).interfaceId;
  }
}
