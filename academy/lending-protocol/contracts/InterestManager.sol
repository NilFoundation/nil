// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title InterestManager
/// @dev The InterestManager contract is responsible for providing the interest rate to be used in the lending protocol.
contract InterestManager {
    /// @notice Fetches the current interest rate.
    /// @dev In this basic implementation, the interest rate is fixed at 5%.
    /// In a real-world scenario, this could be replaced with a dynamic calculation based on market conditions or other factors.
    /// @return uint256 The current interest rate (5% in this case).
    function getInterestRate() public pure returns (uint256) {
        return 5;
    }
}
