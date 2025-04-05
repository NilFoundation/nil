// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title TokenSplitter
 * @notice Distributes received =nil; native tokens (identified by TokenId) to multiple recipients across different shards asynchronously.
 * @dev expects tokens to be transferred to this contract *while* calling splitTokens.
 * @dev Inherits NilTokenBase primarily for convenient access to sendTokenInternal.
 */
contract TokenSplitter is NilBase, NilTokenBase, Ownable, ReentrancyGuard {
    event TokensSplit(
        TokenId indexed tokenId,
        uint256 totalAmount,
        uint256 numRecipients
    );
    event AsyncTransferInitiated(
        TokenId indexed tokenId,
        uint256 indexed shardId,
        address indexed recipient,
        uint256 amount
    );

    //Error messages
    error InvalidAmount();
    error InvalidRecipientAddress();
    error InvalidTokenId();
    error InsufficientTokenBalance();
    error ArrayLengthMismatch();
    error NoRecipientsSpecified();

    receive() external payable {}

    constructor() Ownable(msg.sender) {}

    function splitTokens(
        TokenId _tokenId,
        address[] calldata _recipients,
        uint256[] calldata _amounts
    ) external payable nonReentrant {
        if (_recipients.length == 0) revert NoRecipientsSpecified();
        if (_recipients.length != _amounts.length) revert ArrayLengthMismatch();

        Nil.Token[] memory tokens = Nil.txnTokens();

        uint256 totalAmountToSend = 0;
        for (uint256 i = 0; i < _amounts.length; i++) {
            if (_amounts[i] <= 0) revert InvalidAmount();
            totalAmountToSend += _amounts[i];
        }

        if (tokens[0].amount < totalAmountToSend)
            revert InsufficientTokenBalance();

        for (uint256 i = 0; i < _recipients.length; i++) {
            address recipient = _recipients[i];
            if (_recipients[i] == address(0)) revert InvalidRecipientAddress();

            uint256 amount = _amounts[i];
            uint256 shardId = Nil.getShardId(recipient);

            sendTokenInternal(recipient, _tokenId, amount);

            emit AsyncTransferInitiated(_tokenId, shardId, recipient, amount);
        }

        emit TokensSplit(_tokenId, totalAmountToSend, _recipients.length);
    }

    function withdrawStuckTokens(
        TokenId _tokenId,
        address _to
    ) external onlyOwner {
        uint256 balance = Nil.tokenBalance(address(this), _tokenId);
        if (balance == 0) revert InsufficientTokenBalance();
        sendTokenInternal(_to, _tokenId, balance);
    }
}
