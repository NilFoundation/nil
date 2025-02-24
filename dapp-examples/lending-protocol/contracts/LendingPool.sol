// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@nilfoundation/smart-contracts/contracts/Nil.sol";
import "@nilfoundation/smart-contracts/contracts/NilTokenBase.sol";

contract LendingPool is NilBase, NilTokenBase {
     address public globalLedger;
     address public interestManager;
     address public oracle;
     TokenId public usdt;
     TokenId public eth;
 
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
 
     function deposit() public payable {
         Nil.Token[] memory tokens = Nil.txnTokens();
         // require(tokens.length == 1, "Only one token at a time");
         // require(tokens[0].id == usdt || tokens[0].id == eth, "Invalid token");
 
         bytes memory callData = abi.encodeWithSignature(
             "recordDeposit(address,address,uint256)",
             msg.sender,
             tokens[0].id,
             tokens[0].amount
         );
           Nil.asyncCall(globalLedger, address(this), 0, callData);
     }
 
     function borrow(uint256 amount, TokenId borrowToken) public payable {
         require(borrowToken == usdt || borrowToken == eth, "Invalid token");
         require(Nil.tokenBalance(address(this), borrowToken) >= amount, "Insufficient funds");
         TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;
         bytes memory callData = abi.encodeWithSignature("getPrice(address)", borrowToken);
         bytes memory context = abi.encodeWithSelector(
             this.processLoan.selector, msg.sender, amount, borrowToken, collateralToken
         );
         Nil.sendRequest(oracle, 0, 9_000_000, context, callData);
     }
 
     function processLoan(bool success, bytes memory returnData, bytes memory context) public payable {
         require(success, "Oracle call failed");
 
         (address borrower, uint256 amount, TokenId borrowToken, TokenId collateralToken) =
             abi.decode(context, (address, uint256, TokenId, TokenId));
 
         uint256 borrowTokenPrice = abi.decode(returnData, (uint256));
         uint256 loanValueInUSD = amount * borrowTokenPrice;
         uint256 requiredCollateral = (loanValueInUSD * 120) / 100;
 
         bytes memory ledgerCallData = abi.encodeWithSignature(
             "getDeposit(address,address)", borrower, collateralToken
         );
         bytes memory ledgerContext = abi.encodeWithSelector(
             this.finalizeLoan.selector, borrower, amount, borrowToken, requiredCollateral
         );
         Nil.sendRequest(globalLedger, 0, 6_000_000, ledgerContext, ledgerCallData);
     }
     // add a check for deposit to required collateral by reducing deposit from required collateral in globalLedger
     function finalizeLoan(bool success, bytes memory returnData, bytes memory context) public payable {
         require(success, "Ledger call failed");
 
         (address borrower, uint256 amount, TokenId borrowToken, uint256 requiredCollateral) =
             abi.decode(context, (address, uint256, TokenId, uint256));
 
         (uint256 userCollateral) = abi.decode(returnData, (uint256));
         require(userCollateral >= requiredCollateral, "Insufficient collateral");
 
         bytes memory recordLoanCallData = abi.encodeWithSignature(
             "recordLoan(address,address,uint256)", borrower, borrowToken, amount
         );
         Nil.asyncCall(globalLedger, address(this), 0, recordLoanCallData);
 
         sendTokenInternal(borrower, borrowToken, amount);
     }
 
     function repayLoan() public payable {
         Nil.Token[] memory tokens = Nil.txnTokens();
         bytes memory callData = abi.encodeWithSignature("getLoanDetails(address)", msg.sender);
         bytes memory context = abi.encodeWithSelector(this.handleRepayment.selector, msg.sender, tokens[0].amount);
         Nil.sendRequest(globalLedger, 0, 11_000_000, context, callData);
     }
 
     function handleRepayment(bool success, bytes memory returnData, bytes memory context) public payable {
         require(success, "Ledger call failed");
 
         (address borrower, uint256 sentAmount)= abi.decode(context, (address, uint256));
         (uint256 amount, TokenId token) = abi.decode(returnData, (uint256, TokenId));

         require(amount > 0, "No active loan");
 
         bytes memory interestCallData = abi.encodeWithSignature("getInterestRate()");
         bytes memory interestContext = abi.encodeWithSelector(
             this.processRepayment.selector, borrower, amount, token, sentAmount
         );
         Nil.sendRequest(interestManager, 0, 8_000_000, interestContext, interestCallData);
     }
 
     function processRepayment(bool success, bytes memory returnData, bytes memory context) public payable {
     require(success, "Interest rate call failed");
 
     (address borrower, uint256 amount, TokenId token, uint256 sentAmount) = abi.decode(context, (address, uint256, TokenId, uint256));
     uint256 interestRate = abi.decode(returnData, (uint256));
     uint256 totalRepayment = amount + ((amount * interestRate) / 100);
 
     require(sentAmount >= totalRepayment, "Insufficient funds");
 
     bytes memory clearLoanCallData = abi.encodeWithSignature("recordLoan(address,address,uint256)", borrower, token, 0);
     bytes memory releaseCollateralContext = abi.encodeWithSelector(this.releaseCollateral.selector, borrower, token);
     
     Nil.sendRequest(globalLedger, 0, 6_000_000, releaseCollateralContext, clearLoanCallData);
 }
 function releaseCollateral(bool success, bytes memory returnData, bytes memory context) public payable {
     require(success, "Loan clearing failed");
    // Silence unused variable warning
    returnData;
     (address borrower, TokenId borrowToken) = abi.decode(context, (address, TokenId));
 
     // Determine collateral token (usdt <-> ETH)
     TokenId collateralToken = (borrowToken == usdt) ? eth : usdt;
 
     // Get collateral amount from GlobalLedger
     bytes memory getCollateralCallData = abi.encodeWithSignature("getDeposit(address,address)", borrower, collateralToken);
     bytes memory sendCollateralContext = abi.encodeWithSelector(this.sendCollateral.selector, borrower, collateralToken);
 
     Nil.sendRequest(globalLedger, 0, 3_50_000, sendCollateralContext, getCollateralCallData);
 }
 function sendCollateral(bool success, bytes memory returnData, bytes memory context) public payable {
     require(success, "Failed to retrieve collateral");
 
     (address borrower, TokenId collateralToken) = abi.decode(context, (address, TokenId));
     uint256 collateralAmount = abi.decode(returnData, (uint256));
     
     require(collateralAmount > 0, "No collateral to release");
     require(Nil.tokenBalance(address(this), collateralToken) >= collateralAmount, "Insufficient funds, it fails here");
 
     sendTokenInternal(borrower, collateralToken, collateralAmount);
 }
 
 }
