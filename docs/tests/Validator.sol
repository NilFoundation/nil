// SPDX-License-Identifier: MIT

//startValidator
pragma solidity ^0.8.9;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
contract Validator {
    using Nil for address;

    function validate(
        address participantOne,
        address participantTwo
    ) public returns (bool) {
      bool isValidated = (participantOne != participantTwo);
      return isValidated;
    }
}
//endValidator
