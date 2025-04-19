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

  event ShardIdSet(uint256 indexed oldShardId, uint256 indexed newShardId);

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
   * @param router The address of the router.
   */
  function setRouter(address router) external;

  /**
   * @notice Sets the messenger address.
   * @param messenger The address of the messenger.
   */
  function setMessenger(address messenger) external;

  function setCounterpartyBridge(address counterpartyBridgeAddress) external;

  /**
   * @notice Sets the shardId for L2 contracts.
   * @param shardId The shardId for the L2 contracts.
   */
  function setShardId(uint256 shardId) external;

  /**
   * @notice Sets the NilGasPriceOracle address.
   * @param nilGasPriceOracle The address of the NilGasPriceOracle.
   */
  function setNilGasPriceOracle(address nilGasPriceOracle) external;

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
  function claimFailedDeposit(bytes32 messageHash, uint256 merkleTreeLeafIndex, bytes32[] memory claimProof) external;

  /*//////////////////////////////////////////////////////////////////////////
                             PUBLIC CONSTANT FUNCTIONS
    //////////////////////////////////////////////////////////////////////////*/

  /// @notice The shardId of the NilChain where L2 Contracts are deployed. default is 1.
  function shardId() external view returns (uint256);

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
