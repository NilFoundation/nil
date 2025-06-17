// SPDX-License-Identifier: MIT
pragma solidity ^0.8.21;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";

/**
 * @title SwapMatch
 * @author =nil; Foundation
 * @notice The contract matches swap requests and performs token swaps.
 * On a successful match, the desired tokens are sent to the swap originators.
 * The contract also calculates token excesses and sends them back on a successful match.
 */
contract SwapMatch is NilBase {
    //startContractStructsAndProperties
    /**
     * @notice The struct representing a successful swap.
     */
    struct Swap {
        address firstParty;
        address secondParty;
        TokenId firstTokenId;
        TokenId secondTokenId;
        uint256 firstTokenExchanged;
        uint256 secondTokenExchanged;
    }

    /**
     * @notice The struct representing a single swap request.
     */
    struct SwapRequest {
        address initiator;
        Nil.Token token;
        TokenId secondTokenId;
        uint256 desiredSecondTokenAmount;
        bool isMatched;
    }

    /**
     * @notice A map of all successful swaps.
     */
    mapping(uint256 => Swap) public swaps;

    /**
     * @notice A map of all existing swap requests.
     */
    mapping(uint256 => SwapRequest) public swapRequests;

    /**
     * @dev Counters for swaps and swap requests.
     */
    uint256 private swapCounter = 0;
    uint256 private swapRequestCounter = 0;

    //endContractStructsAndProperties

    //startContractEvents

    //Events that are emitted at various stages of the swap process.
    event NewSwapRequest(uint256 indexed swapRequestNumber, address indexed);
    event NewSwap(uint256 indexed swapNumber, address indexed, address indexed);

    event SwapFinalized(
        uint256 indexed swapNumber,
        address indexed,
        address indexed
    );

    //endContractEvents

    //startPlaceSwapRequest

    /**
     * @notice Places a new swap request. Begins searching for a match after the request is created.
     * When calling this function, some tokens have to be passed in the transaction, otherwise it will fail.
     * @param _desiredSecondTokenAmount The desired amount of tokens specified by the swap originator.
     * @param _secondTokenId The ID of the token that the swap originator wants to receive.
     */
    function placeSwapRequest(
        uint256 _desiredSecondTokenAmount,
        TokenId _secondTokenId
    ) public {
        //Create a new swap request
        SwapRequest memory newSwapRequest = SwapRequest({
            initiator: Nil.msgSender(),
            token: Nil.txnTokens()[0],
            secondTokenId: _secondTokenId,
            desiredSecondTokenAmount: _desiredSecondTokenAmount,
            isMatched: false
        });
        //Update the map, the counter, and emit the event
        swapRequests[swapRequestCounter] = newSwapRequest;
        emit NewSwapRequest(swapRequestCounter, Nil.msgSender());
        swapRequestCounter++;

        //Begin searching for a match
        matchSwapRequests(newSwapRequest);
    }

    //endPlaceSwapRequest

    //startMatchSwapRequests

    /**
     * @notice Matches a swap request with another swap request.
     * On a successful match, begins the withdrawal process.
     * @param swapRequest The swap request to match with another swap request.
     */
    function matchSwapRequests(SwapRequest memory swapRequest) private {
        //Iterate over all registered swap requests
        for (uint256 i = 0; i < swapRequestCounter; i++) {
            SwapRequest storage possibleCandidate = swapRequests[i];
            /**
             * @notice Conducts four checks:
             * If a possible match stores the desired token
             * If a possible match stores a sufficient amount of the desired token
             * If the swap request can satisfy the amount requested by a possible match
             * If a possible match has not yet been matched
             */
            if (
                possibleCandidate.token.id == swapRequest.secondTokenId &&
                possibleCandidate.token.amount >=
                swapRequest.desiredSecondTokenAmount &&
                swapRequest.token.amount >=
                possibleCandidate.desiredSecondTokenAmount &&
                possibleCandidate.isMatched == false
            ) {
                //Create two new Nil.Token objects
                Nil.Token memory tokensPaidToSwapOriginator = Nil.Token({
                    id: possibleCandidate.token.id,
                    amount: swapRequest.desiredSecondTokenAmount
                });
                Nil.Token memory tokensPaidToMatchedSwapOriginator = Nil.Token({
                    id: swapRequest.token.id,
                    amount: possibleCandidate.desiredSecondTokenAmount
                });

                //On a successful match, begin sending the tokens
                sendTokensAndProcessExcess(
                    swapRequest,
                    possibleCandidate,
                    tokensPaidToSwapOriginator,
                    tokensPaidToMatchedSwapOriginator
                );

                //Update the swap requests
                swapRequest.isMatched = true;
                possibleCandidate.isMatched = true;

                //Create a new swap object, add it to the map and update the counter
                Swap memory newSwap = Swap({
                    firstParty: swapRequest.initiator,
                    secondParty: possibleCandidate.initiator,
                    firstTokenId: swapRequest.token.id,
                    secondTokenId: possibleCandidate.token.id,
                    firstTokenExchanged: tokensPaidToSwapOriginator.amount,
                    secondTokenExchanged: tokensPaidToMatchedSwapOriginator
                        .amount
                });

                swaps[swapCounter] = newSwap;
                emit NewSwap(
                    swapCounter,
                    swapRequest.initiator,
                    possibleCandidate.initiator
                );
                swapCounter++;
            }
        }
    }

    //endMatchSwapRequests

    //startSendTokensAndProcessExcess

    /**
     * @notice Sends tokens to both parties participating in the swap and refunds excesses.
     * @param swapRequest The swap request that has been matched.
     * @param matchedSwapRequest The swap request which match the previous swap request.
     * @param tokensPaidToSwapOriginator The tokens paid to the originator of the swap request.
     * @param tokensPaidToMatchedSwapOriginator The tokens paid to the originator of the matched swap request.
     */
    function sendTokensAndProcessExcess(
        SwapRequest memory swapRequest,
        SwapRequest memory matchedSwapRequest,
        Nil.Token memory tokensPaidToSwapOriginator,
        Nil.Token memory tokensPaidToMatchedSwapOriginator
    ) private async(2_000_000) {
        //Calculate possible excesses
        uint256 excessToSwapOriginator = swapRequest.token.amount -
            matchedSwapRequest.desiredSecondTokenAmount;
        uint256 excessToMatchedSwapOriginator = matchedSwapRequest
            .token
            .amount - swapRequest.desiredSecondTokenAmount;

        //Create two arrays of Nil.Token storing the desired amounts and excesses
        Nil.Token[] memory firstTokens = new Nil.Token[](2);
        firstTokens[0].id = tokensPaidToSwapOriginator.id;
        firstTokens[0].amount = tokensPaidToSwapOriginator.amount;
        firstTokens[1].id = swapRequest.token.id;
        firstTokens[1].amount = excessToSwapOriginator;
        Nil.Token[] memory secondTokens = new Nil.Token[](2);
        secondTokens[0].id = tokensPaidToMatchedSwapOriginator.id;
        secondTokens[0].amount = tokensPaidToMatchedSwapOriginator.amount;
        secondTokens[1].id = matchedSwapRequest.token.id;
        secondTokens[1].amount = excessToMatchedSwapOriginator;

        //Perform two async calls delivering the tokens
        Nil.asyncCallWithTokens(
            swapRequest.initiator,
            swapRequest.initiator,
            address(this),
            0,
            Nil.FORWARD_REMAINING,
            0,
            firstTokens,
            ""
        );
        Nil.asyncCallWithTokens(
            matchedSwapRequest.initiator,
            matchedSwapRequest.initiator,
            address(this),
            0,
            Nil.FORWARD_REMAINING,
            0,
            secondTokens,
            ""
        );
    }

    //endSendTokensAndProcessExcess
}
