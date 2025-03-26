// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "./VoteShard.sol";

/**
 * @title VoteManager
 * @notice This contract coordinates a sharded voting system by deploying and managing multiple VoteShard contracts.
 * @dev It handles shard deployment, cross-shard eligibility checks, vote forwarding, and final tally aggregation.
 */
contract VoteManager is NilBase, NilTokenBase, Ownable {
    /// @notice Number of voting shards.
    uint256 public numShards;

    /// @notice Number of available voting choices.
    uint16 public numChoices;

    /// @notice Timestamp at which voting begins.
    uint256 public startTime;

    /// @notice Timestamp at which voting ends.
    uint256 public endTime;

    /// @dev Mapping of shard ID to deployed VoteShard address.
    mapping(uint256 => address) public voteShards;

    /// @dev Aggregated vote counts across all shards.
    mapping(uint16 => uint256) public voteResults;

    /// @notice Emitted after all voting shards have been deployed.
    event DeploymentComplete(uint256 _numShards, uint256 _numChoices);

    /// @notice Emitted after vote results have been tallied.
    event VotesTallied(uint256 timestamp);

    /// @notice Accepts incoming NIL tokens.
    receive() external payable {}

    /**
     * @notice Initializes the VoteManager with configuration.
     * @param _numShards Number of shards to deploy.
     * @param _numChoices Number of valid voting options.
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
     * @notice Deploys all VoteShard contracts using deterministic CREATE2 addresses.
     * @dev Each shard is deployed with the same voting config but a different salt for uniqueness.
     */
    function deployVotingShards() public payable onlyOwner {
        for (uint256 i = 1; i <= numShards; i++) {
            uint256 salt = generateSalt(msg.sender, i);
            deploy(i, salt);
        }

        emit DeploymentComplete(numShards, numChoices);
    }

    /**
     * @dev Internal function to deploy a single VoteShard using `Nil.asyncDeploy`.
     * @param _shardId Unique identifier for the shard.
     * @param _salt Salt for deterministic deployment.
     */
    function deploy(uint256 _shardId, uint256 _salt) private {
        bytes memory data =
            bytes.concat(type(VoteShard).creationCode, abi.encode(address(this), numChoices, startTime, endTime));

        address voteShard = Nil.asyncDeploy(_shardId, address(this), 0, data, _salt);
        voteShards[_shardId] = voteShard;
    }

    /**
     * @notice Allows users to cast votes via the VoteManager instead of directly interacting with a shard.
     * @dev This uses `Nil.asyncCall` to forward the vote to the corresponding VoteShard.
     * @param _shardId The target shard to forward the vote to.
     * @param _choice The user's selected voting option.
     */
    function voteFromManager(uint256 _shardId, uint16 _choice) public payable {
        require(_choice > 0 && _choice <= numChoices, "Invalid choice");
        require(_shardId > 0 && _shardId <= numShards, "Invalid shard ID");

        address voteShard = voteShards[_shardId];
        bytes memory callData = abi.encodeWithSignature("vote(uint16,address)", _choice, msg.sender);

        Nil.asyncCall(voteShard, msg.sender, 0, callData);
    }

    /**
     * @notice Checks if a user has already voted in any shard other than the specified one.
     * @dev This function is called by a VoteShard before allowing a user to vote.
     * @param shardId The current shard attempting to validate the user.
     * @param voter The address of the user to check.
     * @return canVote True if the voter has not voted in any other shard.
     */
    function checkOtherShards(uint256 shardId, address voter) public payable returns (bool canVote) {
        for (uint256 i = 1; i <= numShards; i++) {
            if (i == shardId) continue;

            address voteShard = voteShards[i];
            (bytes memory result, bool ok) = Nil.awaitCall(
                voteShard, Nil.ASYNC_REQUEST_MIN_GAS, abi.encodeWithSignature("checkHasVoted(address)", voter)
            );

            require(ok, "Async call failed");

            bool hasVoted = abi.decode(result, (bool));
            if (hasVoted) {
                return false;
            }
        }

        return true;
    }

    /**
     * @notice Aggregates and tallies the vote results from all shards.
     * @dev Uses `Nil.awaitCall` to synchronously request results from each shard.
     * Updates the `voteResults` mapping for final results.
     */
    function tallyTotalVotes() public payable {
        for (uint256 i = 1; i <= numShards; i++) {
            address voteShard = voteShards[i];

            (bytes memory temp, bool ok) =
                Nil.awaitCall(voteShard, Nil.ASYNC_REQUEST_MIN_GAS, abi.encodeWithSignature("tallyVotes()"));

            require(ok, "Tally request failed");

            uint256[] memory votes = abi.decode(temp, (uint256[]));

            for (uint16 j = 1; j <= numChoices; j++) {
                voteResults[j] += votes[j];
            }
        }

        emit VotesTallied(block.timestamp);
    }

    /**
     * @notice Required callback for =nil; verifier to accept external inputs.
     * @dev Always returns true. Used by the Nil runtime to validate external calls.
     */
    function verifyExternal(uint256, bytes calldata) external pure returns (bool) {
        return true;
    }

    /**
     * @dev Generates a deterministic salt from a user address and nonce.
     * @param _user The address to base the salt on.
     * @param _nonce A nonce value to guarantee uniqueness.
     * @return A keccak256-hashed salt as uint256.
     */
    function generateSalt(address _user, uint256 _nonce) private pure returns (uint256) {
        return uint256(keccak256(abi.encodePacked(_user, _nonce)));
    }

    /**
     * @notice Retrieves the deployed VoteShard address for a given shard ID.
     * @param _shardId The shard ID.
     * @return The address of the deployed VoteShard.
     */
    function getShardAddress(uint256 _shardId) public view returns (address) {
        return voteShards[_shardId];
    }

    /**
     * @notice Returns the aggregated vote results for all choices.
     * @dev Index 0 is unused; choices start from index 1.
     * @return results An array of vote counts per choice.
     */
    function getVotingResult() public view returns (uint256[] memory results) {
        results = new uint256[](numChoices + 1); // index 0 is unused

        for (uint16 i = 1; i <= numChoices; i++) {
            results[i] = voteResults[i];
        }
    }
}
