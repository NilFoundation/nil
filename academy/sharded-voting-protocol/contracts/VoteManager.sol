// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "./VoteShard.sol";

/**
 * @title VoteManager
 * @dev Manages deployment of voting shards and coordination of vote casting and tallying across shards.
 */
contract VoteManager is NilBase, NilTokenBase, Ownable {
    uint256 public numShards;
    uint16 public numChoices;
    uint256 public startTime;
    uint256 public endTime;

    mapping(uint256 => address) public voteShards;
    mapping(uint16 => uint256) public voteResults;

    event DeploymentComplete(uint256 _numShards, uint256 _numChoices);
    event VotesTallied(uint256 timestamp);

    receive() external payable {}

    /**
     * @dev Initializes the voting manager with shard and voting configuration.
     * @param _numShards Number of vote shards to deploy.
     * @param _numChoices Number of voting options.
     * @param _startTime Timestamp when voting starts.
     * @param _endTime Timestamp when voting ends.
     */
    constructor(uint256 _numShards, uint16 _numChoices, uint256 _startTime, uint256 _endTime)
        payable
        Ownable(msg.sender)
    {
        require(_numShards > 0, "Shards start from 1");
        require(_startTime >= block.timestamp, "Start time must be in the future");
        require(_endTime > _startTime, "End time must be after start time");
        require(_numChoices > 0, "Choices start from 1");

        numShards = _numShards;
        numChoices = _numChoices;
        startTime = _startTime;
        endTime = _endTime;
    }

    /**
     * @notice Deploys all vote shard contracts deterministically using CREATE2.
     */
    function deployVotingShards() public payable onlyOwner {
        for (uint256 i = 1; i <= numShards; i++) {
            uint256 salt = generateSalt(msg.sender, i);
            deploy(i, salt);
        }

        emit DeploymentComplete(numShards, numChoices);
    }

    /**
     * @dev Deploys a single VoteShard contract using Nil.asyncDeploy.
     * @param _shardId ID of the shard.
     * @param _salt Deterministic salt for CREATE deployment.
     */
    function deploy(uint256 _shardId, uint256 _salt) private {
        bytes memory data =
            bytes.concat(type(VoteShard).creationCode, abi.encode(address(this), numChoices, startTime, endTime));
        address voteShard = Nil.asyncDeploy(_shardId, address(this), 0, data, _salt);
        voteShards[_shardId] = voteShard;
    }

    /**
     * @notice Allows users to vote through the VoteManager contract.
     * @param _shardId ID of the VoteShard to cast the vote in.
     * @param _choice Voter's selected option.
     */
    function voteFromManager(uint256 _shardId, uint16 _choice) public payable {
        require(_choice > 0 && _choice <= numChoices, "Invalid choice");
        require(_shardId > 0 && _shardId <= numShards, "Invalid shard ID");

        address voteShard = voteShards[_shardId];
        bytes memory callData = abi.encodeWithSignature("vote(uint16,address)", _choice, msg.sender);

        Nil.asyncCall(voteShard, msg.sender, 0, callData);
    }

    /**
     * @notice Initiates asynchronous tallying across all voting shards.
     */
    function tallyTotalVotes() public payable {
        for (uint256 i = 1; i <= numShards; i++) {
            address voteShard = voteShards[i];
            bytes memory temp;
            bool ok;
            (temp, ok) = Nil.awaitCall(voteShard, Nil.ASYNC_REQUEST_MIN_GAS, abi.encodeWithSignature("tallyVotes()"));

            require(ok == true, "Result not true");

            uint256[] memory votes = abi.decode(temp, (uint256[]));

            for (uint16 j = 1; i <= numChoices; j++) {
                voteResults[j] += votes[j];
            }
        }
        emit VotesTallied(block.timestamp);
    }

    /**
     * @notice Verification hook required by Nil runtime.
     */
    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    /**
     * @dev Generates a deterministic salt using address and nonce.
     * @param _user Address to base the salt on.
     * @param _nonce A nonce value to ensure uniqueness.
     */
    function generateSalt(address _user, uint256 _nonce) private pure returns (uint256) {
        return uint256(keccak256(abi.encodePacked(_user, _nonce)));
    }

    /**
     * @notice Retrieves the address of a deployed VoteShard contract by its shard ID.
     * @dev Useful for checking where a specific shard is deployed.
     * @param _shardId The ID of the shard.
     * @return The address of the corresponding VoteShard contract.
     */
    function getShardAddress(uint256 _shardId) public view returns (address) {
        return voteShards[_shardId];
    }

    /**
     * @notice Returns the current voting results aggregated by the VoteManager.
     * @dev The results array uses index positions corresponding to choice numbers; index 0 is unused.
     * @return results An array where each index represents the total votes for that choice.
     */
    function getVotingResult() public view returns (uint256[] memory results) {
        results = new uint256[](numChoices + 1); // Index 0 is unused

        for (uint16 i = 1; i <= numChoices; i++) {
            results[i] = voteResults[i];
        }
    }
}
