// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

/// @title LendingPool
/// @dev The LendingPool contract facilitates lending and borrowing of tokens and handles collateral management.
/// It interacts with other contracts such as GlobalLedger, InterestManager, and Oracle for tracking deposits, calculating interest, and fetching token prices.
contract LendingPool is NilBase, NilTokenBase {
    address public globalLedger;
    address public interestManager;
    address public oracle;
    TokenId public usdt;
    TokenId public eth;

    /// @notice Constructor to initialize the LendingPool contract with addresses for dependencies.
    /// @dev Sets the contract addresses for GlobalLedger, InterestManager, Oracle, USDT, and ETH tokens.
    /// @param _globalLedger The address of the GlobalLedger contract.
    /// @param _interestManager The address of the InterestManager contract.
    /// @param _oracle The address of the Oracle contract.
    /// @param _usdt The TokenId for USDT.
    /// @param _eth The TokenId for ETH.
    constructor(
        address _globalLedger,
        address _interestManager,
        address _oracle,
        TokenId _usdt,
        TokenId _eth
    ) {
        globalLedger = _globalLedger;
        interestManager = _interestManager;
        oracle = _oracle;
        usdt = _usdt;
        eth = _eth;
    }

    /// @notice Deposit function to deposit tokens into the lending pool.
    /// @dev The deposited tokens are recorded in the GlobalLedger via an asynchronous call.
    function deposit() public payable {
        /// Retrieve the tokens being sent in the transaction
        Nil.Token[] memory tokens = Nil.txnTokens();

        /// @notice Encoding the call to the GlobalLedger to record the deposit
        /// @dev The deposit details (user address, token type, and amount) are encoded for GlobalLedger.
        /// @param callData The encoded call data for recording the deposit in GlobalLedger.
        bytes memory callData = abi.encodeWithSignature(
            "recordDeposit(address,address,uint256)",
            msg.sender,
            tokens[0].id, // The token being deposited (usdt or eth)
            tokens[0].amount // The amount of the token being deposited
        );

        /// @notice Making an asynchronous call to the GlobalLedger to record the deposit
        /// @dev This ensures that the user's deposit is recorded in GlobalLedger asynchronously.
        Nil.asyncCall(globalLedger, address(this), 0, callData);
    }

    /// @notice Borrow function allows a user to borrow tokens (either USDT or ETH).
    /// @dev Ensures sufficient liquidity, checks collateral, and processes the loan after fetching the price from the Oracle.
    /// @param amount The amount of the token to borrow.
    /// @param borrowToken The token the user wants to borrow (either USDT or ETH).
    function borrow(uint256 amount, TokenId borrowToken) public payable {
        /// @notice Ensure the token being borrowed is either USDT or ETH
        /// @dev Prevents invalid token types from being borrowed.
        require(borrowToken == usdt || borrowToken == eth, "Invalid token");

        /// @notice Ensure that the LendingPool has enough liquidity of the requested borrow token
        /// @dev Checks the LendingPool's balance to confirm it has enough tokens to fulfill the borrow request.
        require(
            Nil.tokenBalance(address(this), borrowToken) >= amount,
            "Insufficient funds"
        );

        /// @notice Determine which collateral token will be used (opposite of the borrow token)
        /// @dev Identifies the collateral token by comparing the borrow token.
        TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;

        /// @notice Prepare a call to the Oracle to get the price of the borrow token
        /// @dev The price of the borrow token is fetched from the Oracle to calculate collateral.
        /// @param callData The encoded data to fetch the price from the Oracle.
        bytes memory callData = abi.encodeWithSignature(
            "getPrice(address)",
            borrowToken
        );

        /// @notice Encoding the context to process the loan after the price is fetched
        /// @dev The context contains the borrower’s details, loan amount, borrow token, and collateral token.
        bytes memory context = abi.encodeWithSelector(
            this.processLoan.selector,
            msg.sender,
            amount,
            borrowToken,
            collateralToken
        );

        /// @notice Send a request to the Oracle to get the price of the borrow token.
        /// @dev This request is processed with a fee for the transaction, allowing the system to fetch the token price.
        Nil.sendRequest(oracle, 0, 9_000_000, context, callData);
    }

    /// @notice Callback function to process the loan after the price data is retrieved from Oracle.
    /// @dev Ensures that the borrower has enough collateral, calculates the loan value, and initiates loan processing.
    /// @param success Indicates if the Oracle call was successful.
    /// @param returnData The price data returned from the Oracle.
    /// @param context The context data containing borrower details, loan amount, and collateral token.
    function processLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the Oracle call was successful
        /// @dev Verifies that the price data was successfully retrieved from the Oracle.
        require(success, "Oracle call failed");

        /// @notice Decode the context to extract borrower details, loan amount, and collateral token
        /// @dev Decodes the context passed from the borrow function to retrieve necessary data.
        (
            address borrower,
            uint256 amount,
            TokenId borrowToken,
            TokenId collateralToken
        ) = abi.decode(context, (address, uint256, TokenId, TokenId));

        /// @notice Decode the price data returned from the Oracle
        /// @dev The returned price data is used to calculate the loan value in USD.
        uint256 borrowTokenPrice = abi.decode(returnData, (uint256));
        /// @notice Calculate the loan value in USD
        /// @dev Multiplies the amount by the borrow token price to get the loan value in USD.
        uint256 loanValueInUSD = amount * borrowTokenPrice;
        /// @notice Calculate the required collateral (120% of the loan value)
        /// @dev The collateral is calculated as 120% of the loan value to mitigate risk.
        uint256 requiredCollateral = (loanValueInUSD * 120) / 100;

        /// @notice Prepare a call to GlobalLedger to check the user's collateral balance
        /// @dev Fetches the collateral balance from the GlobalLedger contract to ensure sufficient collateral.
        bytes memory ledgerCallData = abi.encodeWithSignature(
            "getDeposit(address,address)",
            borrower,
            collateralToken
        );

        /// @notice Encoding the context to finalize the loan once the collateral is validated
        /// @dev Once the collateral balance is validated, the loan is finalized and processed.
        bytes memory ledgerContext = abi.encodeWithSelector(
            this.finalizeLoan.selector,
            borrower,
            amount,
            borrowToken,
            requiredCollateral
        );

        /// @notice Send request to GlobalLedger to get the user's collateral
        /// @dev The fee for this request is retained for processing the collateral validation response.
        Nil.sendRequest(
            globalLedger,
            0,
            6_000_000,
            ledgerContext,
            ledgerCallData
        );
    }

    /// @notice Finalize the loan by ensuring sufficient collateral and recording the loan in GlobalLedger.
    /// @dev Verifies that the user has enough collateral, processes the loan, and sends the borrowed tokens to the borrower.
    /// @param success Indicates if the collateral check was successful.
    /// @param returnData The collateral balance returned from the GlobalLedger.
    /// @param context The context containing loan details.
    function finalizeLoan(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the collateral check was successful
        /// @dev Verifies the collateral validation result from GlobalLedger.
        require(success, "Ledger call failed");

        /// @notice Decode the context to extract loan details
        /// @dev Decodes the context passed from the processLoan function to retrieve loan data.
        (
            address borrower,
            uint256 amount,
            TokenId borrowToken,
            uint256 requiredCollateral
        ) = abi.decode(context, (address, uint256, TokenId, uint256));

        /// @notice Decode the user's collateral balance from GlobalLedger
        /// @dev Retrieves the user's collateral balance from the GlobalLedger to compare it with the required collateral.
        uint256 userCollateral = abi.decode(returnData, (uint256));

        /// @notice Check if the user has enough collateral to cover the loan
        /// @dev Ensures the borrower has sufficient collateral before proceeding with the loan.
        require(
            userCollateral >= requiredCollateral,
            "Insufficient collateral"
        );

        /// @notice Record the loan in GlobalLedger
        /// @dev The loan details are recorded in the GlobalLedger contract.
        bytes memory recordLoanCallData = abi.encodeWithSignature(
            "recordLoan(address,address,uint256)",
            borrower,
            borrowToken,
            amount
        );
        Nil.asyncCall(globalLedger, address(this), 0, recordLoanCallData);

        /// @notice Send the borrowed tokens to the borrower
        /// @dev Transfers the loan amount to the borrower's address after finalizing the loan.
        sendTokenInternal(borrower, borrowToken, amount);
    }

    /// @notice Repay loan function called by the borrower to repay their loan.
    /// @dev Initiates the repayment process by retrieving the loan details from GlobalLedger.
    function repayLoan() public payable {
        /// @notice Retrieve the tokens being sent in the transaction
        /// @dev Retrieves the tokens involved in the repayment.
        Nil.Token[] memory tokens = Nil.txnTokens();

        /// @notice Prepare to query the loan details from GlobalLedger
        /// @dev Fetches the loan details of the borrower to proceed with repayment.
        bytes memory callData = abi.encodeWithSignature(
            "getLoanDetails(address)",
            msg.sender
        );

        /// @notice Encoding the context to handle repayment after loan details are fetched
        /// @dev Once the loan details are retrieved, the repayment amount is processed.
        bytes memory context = abi.encodeWithSelector(
            this.handleRepayment.selector,
            msg.sender,
            tokens[0].amount
        );

        /// @notice Send request to GlobalLedger to fetch loan details
        /// @dev Retrieves the borrower’s loan details before proceeding with the repayment.
        Nil.sendRequest(globalLedger, 0, 11_000_000, context, callData);
    }

    /// @notice Handle the loan repayment, calculate the interest, and update GlobalLedger.
    /// @dev Calculates the total repayment (principal + interest) and updates the loan status in GlobalLedger.
    /// @param success Indicates if the loan details retrieval was successful.
    /// @param returnData The loan details returned from the GlobalLedger.
    /// @param context The context containing borrower and repayment details.
    function handleRepayment(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the GlobalLedger call was successful
        /// @dev Verifies that the loan details were successfully retrieved from the GlobalLedger.
        require(success, "Ledger call failed");

        /// @notice Decode context and loan details
        /// @dev Decodes the context and the return data to retrieve the borrower's loan details.
        (address borrower, uint256 sentAmount) = abi.decode(
            context,
            (address, uint256)
        );
        (uint256 amount, TokenId token) = abi.decode(
            returnData,
            (uint256, TokenId)
        );

        /// @notice Ensure the borrower has an active loan
        /// @dev Ensures the borrower has an outstanding loan before proceeding with repayment.
        require(amount > 0, "No active loan");

        /// @notice Request the interest rate from the InterestManager
        /// @dev Fetches the current interest rate for the loan from the InterestManager contract.
        bytes memory interestCallData = abi.encodeWithSignature(
            "getInterestRate()"
        );
        bytes memory interestContext = abi.encodeWithSelector(
            this.processRepayment.selector,
            borrower,
            amount,
            token,
            sentAmount
        );

        /// @notice Send request to InterestManager to fetch interest rate
        /// @dev This request fetches the interest rate that will be used to calculate the total repayment.
        Nil.sendRequest(
            interestManager,
            0,
            8_000_000,
            interestContext,
            interestCallData
        );
    }

    /// @notice Process the repayment, calculate the total repayment including interest.
    /// @dev Finalizes the loan repayment, ensuring the borrower has sent sufficient funds.
    /// @param success Indicates if the interest rate call was successful.
    /// @param returnData The interest rate returned from the InterestManager.
    /// @param context The context containing repayment details.
    function processRepayment(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the interest rate call was successful
        /// @dev Verifies that the interest rate retrieval was successful.
        require(success, "Interest rate call failed");

        /// @notice Decode the repayment details and the interest rate
        /// @dev Decodes the repayment context and retrieves the interest rate for loan repayment.
        (
            address borrower,
            uint256 amount,
            TokenId token,
            uint256 sentAmount
        ) = abi.decode(context, (address, uint256, TokenId, uint256));

        /// @notice Decode the interest rate from the response
        /// @dev Decodes the interest rate received from the InterestManager contract.
        uint256 interestRate = abi.decode(returnData, (uint256));
        /// @notice Calculate the total repayment amount (principal + interest)
        /// @dev Adds the interest to the principal to calculate the total repayment due.
        uint256 totalRepayment = amount + ((amount * interestRate) / 100);

        /// @notice Ensure the borrower has sent sufficient funds for the repayment
        /// @dev Verifies that the borrower has provided enough funds to repay the loan in full.
        require(sentAmount >= totalRepayment, "Insufficient funds");

        /// @notice Clear the loan and release collateral
        /// @dev Marks the loan as repaid and releases any associated collateral back to the borrower.
        bytes memory clearLoanCallData = abi.encodeWithSignature(
            "recordLoan(address,address,uint256)",
            borrower,
            token,
            0 // Mark the loan as repaid
        );
        bytes memory releaseCollateralContext = abi.encodeWithSelector(
            this.releaseCollateral.selector,
            borrower,
            token
        );

        /// @notice Send request to GlobalLedger to update the loan status
        /// @dev Updates the loan status to indicate repayment completion in the GlobalLedger.
        Nil.sendRequest(
            globalLedger,
            0,
            6_000_000,
            releaseCollateralContext,
            clearLoanCallData
        );
    }

    /// @notice Release the collateral after the loan is repaid.
    /// @dev Sends the collateral back to the borrower after confirming the loan is fully repaid.
    /// @param success Indicates if the loan clearing was successful.
    /// @param returnData The collateral data returned from the GlobalLedger.
    /// @param context The context containing borrower and collateral token.
    function releaseCollateral(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the loan clearing was successful
        /// @dev Verifies the result of clearing the loan in the GlobalLedger.
        require(success, "Loan clearing failed");

        /// @notice Silence unused variable warning
        /// @dev A placeholder for unused variables to avoid compiler warnings.
        returnData;

        /// @notice Decode context for borrower and collateral token
        /// @dev Decodes the context passed from the loan clearing function to retrieve the borrower's details.
        (address borrower, TokenId borrowToken) = abi.decode(
            context,
            (address, TokenId)
        );

        /// @notice Determine the collateral token (opposite of borrow token)
        /// @dev Identifies the token being used as collateral based on the borrow token.
        TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;

        /// @notice Request collateral amount from GlobalLedger
        /// @dev Retrieves the amount of collateral associated with the borrower from the GlobalLedger.
        bytes memory getCollateralCallData = abi.encodeWithSignature(
            "getDeposit(address,address)",
            borrower,
            collateralToken
        );

        /// @notice Context to send collateral to the borrower
        /// @dev After confirming the collateral balance, it is returned to the borrower.
        bytes memory sendCollateralContext = abi.encodeWithSelector(
            this.sendCollateral.selector,
            borrower,
            collateralToken
        );

        /// @notice Send request to GlobalLedger to retrieve the collateral
        /// @dev This request ensures that the correct collateral is available for release.
        Nil.sendRequest(
            globalLedger,
            0,
            3_50_000,
            sendCollateralContext,
            getCollateralCallData
        );
    }

    /// @notice Send the collateral back to the borrower.
    /// @dev Ensures there is enough collateral to release and then sends the funds back to the borrower.
    /// @param success Indicates if the collateral retrieval was successful.
    /// @param returnData The amount of collateral available.
    /// @param context The context containing borrower and collateral token.
    function sendCollateral(
        bool success,
        bytes memory returnData,
        bytes memory context
    ) public payable {
        /// @notice Ensure the collateral retrieval was successful
        /// @dev Verifies that the request to retrieve the collateral was successful.
        require(success, "Failed to retrieve collateral");

        /// @notice Decode the collateral details
        /// @dev Decodes the context passed from the releaseCollateral function to retrieve collateral details.
        (address borrower, TokenId collateralToken) = abi.decode(
            context,
            (address, TokenId)
        );
        uint256 collateralAmount = abi.decode(returnData, (uint256));

        /// @notice Ensure there's collateral to release
        /// @dev Verifies that there is enough collateral to be released.
        require(collateralAmount > 0, "No collateral to release");

        /// @notice Ensure sufficient balance in the LendingPool to send collateral
        /// @dev Verifies that the LendingPool has enough collateral to send to the borrower.
        require(
            Nil.tokenBalance(address(this), collateralToken) >=
                collateralAmount,
            "Insufficient funds"
        );

        /// @notice Send the collateral tokens to the borrower
        /// @dev Executes the transfer of collateral tokens back to the borrower.
        sendTokenInternal(borrower, collateralToken, collateralAmount);
    }
}
