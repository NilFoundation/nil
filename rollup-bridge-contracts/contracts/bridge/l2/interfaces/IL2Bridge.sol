// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IBridge } from "../../interfaces/IBridge.sol";

interface IL2Bridge is IBridge {
  error ErrorInvalidCounterpartyBridge();

  /// @notice Thrown when the L1 token address is invalid
  error ErrorInvalidL1TokenAddress();

  /// @notice Thrown when the token address is invalid
  error ErrorInvalidTokenAddress();

  /// @notice Thrown when the L1 token address does not match the expected address
  error ErrorL1TokenAddressMismatch();

  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS   
    //////////////////////////////////////////////////////////////////////////*/

  event CounterpartyBridgeSet(address indexed counterpartyBridge, address indexed newCounterpartyBridge);

  event BridgeMessengerSet(address indexed messenger, address indexed newMessenger);

  event BridgeRouterSet(address indexed router, address indexed newRouter);

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATION FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  function setRouter(address routerAddress) external;

  function setMessenger(address messengerAddress) external;

  function setCounterpartyBridge(address counterpartyBridgeAddress) external;

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice The address of L1BridgeRouter/L2BridgeRouter contract.
  function router() external view returns (address);

  /// @notice The address of Bridge contract on other side (for L1Bridge it would be the bridge-address on L2 and for
  /// L2Bridge this would be the bridge-address on L1)
  function counterpartyBridge() external view returns (address);

  /// @notice The address of corresponding L1NilMessenger/L2NilMessenger contract.
  function messenger() external view returns (address);

  function setPause(bool _status) external;

  function transferOwnershipRole(address newOwner) external;
}
