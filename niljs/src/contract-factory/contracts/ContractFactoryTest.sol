// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;
import "@nilfoundation/smart-contracts/contracts/Nil.sol";

contract ContractFactoryTest {
    uint256 public counter;
    uint256 last_received;
    uint256 last_token_amount;

    constructor(uint256 start) {
        counter = start;
    }

    function increment() public {
        counter++;
    }

    function getCounter() public view returns (uint256) {
        return counter;
    }

    function setCounter(uint256 _counter) public {
        counter = _counter;
    }

    function receiveMoney() public payable {
        last_received = msg.value;
    }

    function getReceived() public view returns (uint256) {
        return last_received;
    }

    function receiveToken() public payable {
        Nil.Token[] memory tokens = Nil.txnTokens();  // TODO: [PoC] Tokens remove it
        last_token_amount = tokens[0].amount;
    }
}
