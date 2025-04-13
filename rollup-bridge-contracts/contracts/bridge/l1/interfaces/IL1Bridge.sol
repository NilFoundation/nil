// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import { IBridge } from "../../interfaces/IBridge.sol";
import { INilGasPriceOracle } from "./INilGasPriceOracle.sol";

interface IL1Bridge is IBridge {
  /*//////////////////////////////////////////////////////////////////////////
                             EVENTS
    //////////////////////////////////////////////////////////////////////////*/

  event CounterpartyBridgeSet(address indexed counterpartyBridge, address indexed newCounterpartyBridge);

  event BridgeMessengerSet(address indexed messenger, address indexed newMessenger);

  event BridgeRouterSet(address indexed router, address indexed newRouter);

  event NilGasPriceOracleSet(address indexed nilGasPriceOracle, address indexed newNilGasPriceOracle);

  /**
   * @notice Emitted when a deposit is claimed.
   * @param messageHash The hash of the deposit message.
   * @param tokenAddress The address of the token.
   * @param depositorAddress The address of the depositor.
   * @param l1DepositAmount The amount of the deposit on L1.
   */
  event DepositClaimed(
    bytes32 indexed messageHash,
    address indexed tokenAddress,
    address indexed depositorAddress,
    uint256 l1DepositAmount
  );

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC MUTATION FUNCTIONS   
    //////////////////////////////////////////////////////////////////////////*/

  /**
   * @notice Sets the router address.
   * @param _router The address of the router.
   */
  function setRouter(address _router) external;

  /**
   * @notice Sets the messenger address.
   * @param _messenger The address of the messenger.
   */
  function setMessenger(address _messenger) external;

  function setCounterpartyBridge(address counterpartyBridgeAddress) external;

  /**
   * @notice Sets the NilGasPriceOracle address.
   * @param _nilGasPriceOracle The address of the NilGasPriceOracle.
   */
  function setNilGasPriceOracle(address _nilGasPriceOracle) external;

  /**
   * @notice Cancels a deposit.
   * @param messageHash The hash of the deposit message to be canceled.
   */
  function cancelDeposit(bytes32 messageHash) external;

  /**
   * @notice Claims a failed deposit by verifying the Merkle proof.
   * @param messageHash The hash of the deposit message.
   * @param claimProof The Merkle proof as an array of bytes32.
   */
  function claimFailedDeposit(bytes32 messageHash, bytes32[] memory claimProof) external;

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

  /// @notice The address of the nilGasPriceOracle contract which contains the maxFeePerGas and maxPriorityFeePerGas
  function nilGasPriceOracle() external view returns (address);
}
