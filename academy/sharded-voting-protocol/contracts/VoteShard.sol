// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

/**
 * @title VoteShard
 * @dev A shard contract for collecting and tallying votes on a subset of users.
 */
contract VoteShard {
    uint16 public numChoices;
    uint256 public startTime;
    uint256 public endTime;
    address public voteManager;

    mapping(address => uint16) public options;
    mapping(address => bool) public hasVoted;
    mapping(uint16 => uint256) public totalVotes;

    event VoteCast(address indexed voter, uint16 indexed choice);
    event VotingClosed(uint256 endTime);

    /**
     * @dev Initializes the shard with voting parameters.
     * @param _voteManager Address of the VoteManager contract.
     * @param _numChoices Number of available voting choices.
     * @param _startTime Voting start timestamp.
     * @param _endTime Voting end timestamp.
     */
    constructor(address _voteManager, uint16 _numChoices, uint256 _startTime, uint256 _endTime) {
        require(_startTime >= block.timestamp, "Start time must be in the future");
        require(_endTime > _startTime, "End time must be after start time");
        require(_numChoices > 0, "Choices start from 1");

        numChoices = _numChoices;
        startTime = _startTime;
        endTime = _endTime;
        voteManager = _voteManager;
    }

    /**
     * @notice Casts a vote for a given choice.
     * @param _choice The index of the selected choice (1-based).
     * @param _sender The original voter's address.
     */
    function vote(uint16 _choice, address _sender) public duringVote {
        require(_choice > 0 && _choice <= numChoices, "Invalid choice");

        address voter;

        if (isContract(msg.sender)) {
            require(msg.sender == voteManager, "Only voteManager can call from contract");
            voter = _sender;
        } else {
            require(_sender == msg.sender, "Sender must match caller");
            voter = msg.sender;
        }

        require(!hasVoted[voter], "Already voted");

        options[voter] = _choice;
        hasVoted[voter] = true;
        totalVotes[_choice] += 1;

        emit VoteCast(voter, _choice);
    }

    /**
     * @notice Tally all votes for each choice.
     * @return results Array containing total votes per choice.
     * @dev Index 0 is unused; results[1] corresponds to choice 1.
     */
    function tallyVotes() external view returns (uint256[] memory results) {
        results = new uint256[](numChoices + 1); // Index 0 is unused

        for (uint16 i = 1; i <= numChoices; i++) {
            results[i] = totalVotes[i];
        }
    }

    /**
     * @dev Modifier to restrict functions to active voting window.
     */
    modifier duringVote() {
        require(block.timestamp >= startTime && block.timestamp <= endTime, "Voting is not active");
        _;
    }

    /**
     * @notice check if an address is a contract
     * @return results Array containing total votes per choice.
     * @param _addr address to check
     */
    function isContract(address _addr) internal view returns (bool) {
        if (_addr.code.length == 0) {
            return false;
        } else {
            return true;
        }
    }
}
