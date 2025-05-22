import React, { useState, useEffect, useCallback } from "react";
import { encodeFunctionData } from "viem";
import "./App.css";
import { getContract, waitTillCompleted } from "@nilfoundation/niljs";
import globalLedgerAbi from "./artifacts/globalLedger.json";
import abi from "./artifacts/lendingPool.json";
import constants from "./constants";
import { createClient } from "./utils/createClient";
import { processReceipts } from "./utils/receiptChecker";

/**
 * Example DeFi Frontend Integration with =nil; blockchain
 * This application demonstrates how to build a React frontend that interacts with
 * the =nil; blockchain infrastructure and wallet. Use this as a reference for
 * creating your own decentralized applications with =nil;.
 */
function App() {
  // UI state management
  const [activeTab, setActiveTab] = useState("lend"); // Current active tab (lend, borrow, repay)
  const [amount, setAmount] = useState(""); // User-entered token amount
  const [selectedToken, setSelectedToken] = useState("ETH"); // Currently selected token
  const [walletConnected, setWalletConnected] = useState(false); // Wallet connection status
  const [account, setAccount] = useState(""); // Connected wallet address
  const [lastTxHash, setLastTxHash] = useState(""); // Most recent transaction hash
  const [isLoading, setIsLoading] = useState(false); // Transaction loading state

  // User financial information
  const [borrowableAmount, setBorrowableAmount] = useState(0); // Maximum amount user can borrow
  const [repayableAmount, setRepayableAmount] = useState(0); // Amount user needs to repay
  const [isLoadingAmounts, setIsLoadingAmounts] = useState(false); // Loading state for financial data

  const [publicClient, setPublicClient] = useState(createClient());
  // Contract information
  // Address of lending pool derived in the logs by running the hardhat task in the parent folder: npx hardhat run-lending-protocol
  const contractAddress = constants.contractAddress;
  const contractABI = abi.abi; // Lending pool contract ABI

  // Token addresses on the blockchain
  const ETH = constants.ETH;
  const USDT = constants.USDT;

  /**
   * Connects to the =nil; wallet and retrieves the user's account
   * @async
   * @returns {Promise<void>}
   */
  const connectWallet = async () => {
    try {
      if (window.nil) {
        const accounts = await window.nil.request({
          method: "eth_requestAccounts",
        });
        console.log("Connected account:", accounts[0]);
        setAccount(accounts[0]);
        setWalletConnected(true);
      } else {
        alert("Please install =nil; wallet from the chrome store!");
      }
    } catch (error) {
      console.error("Error connecting wallet:", error);
    }
  };

  /**
   * Fetches borrowable and repayable amounts for the connected user
   * Uses the global ledger contract to retrieve deposit information and loan details
   * @async
   * @returns {Promise<void>}
   */
  const fetchUserAmounts = useCallback(async () => {
    if (!walletConnected || !account) return;

    setIsLoadingAmounts(true);
    try {
      const token = selectedToken === "ETH" ? ETH : USDT;
      const client = await (await publicClient).publicClient;

      // Get Contract instance of lending pool
      const contract = await getContract({
        abi: contractABI,
        address: contractAddress,
        client,
      });

      // Get address of the global ledger contract
      const globalLedger = await contract.read.globalLedger();

      // Create contract instance for global ledger
      const globalLedgerContract = await getContract({
        abi: globalLedgerAbi.abi,
        address: globalLedger,
        client,
      });

      // Fetch user's deposited amounts for both tokens
      const ethDepositAmount = await globalLedgerContract.read.getDeposit([
        account,
        ETH,
      ]);
      const usdtDepositAmount = await globalLedgerContract.read.getDeposit([
        account,
        USDT,
      ]);

      console.log("ETH deposit", ethDepositAmount);
      console.log("USDT Deposit", usdtDepositAmount);

      // Fixed exchange rate for token conversion
      const exchangeRate = 2; // 1 ETH = 2 USDT

      // Calculate total value in the selected token's denomination
      let totalValueInSelectedToken = 0;

      // Convert deposits to selected token value
      if (selectedToken === "ETH") {
        // Convert USDT to ETH value (divide by exchange rate)
        const usdtDeposit = Number(usdtDepositAmount);
        const borrowable = usdtDeposit / exchangeRate;
        totalValueInSelectedToken = borrowable;
      } else {
        // Convert ETH to USDT value (multiply by exchange rate)
        const ethDeposit = Number(ethDepositAmount);
        const borrowable = Number(ethDeposit) * exchangeRate;
        totalValueInSelectedToken = borrowable;
      }

      // Calculate borrowable amount - 80% of total deposit value (collateralization ratio)
      const borrowableResult = totalValueInSelectedToken * 0.8;

      // Fetch the user's existing loan details
      const loanDetails = await globalLedgerContract.read.getLoanDetails([
        account,
      ]);

      // Calculate repayable amount with 5% interest
      let repayableResult = 0;
      console.log(loanDetails);
      if (
        loanDetails &&
        Array.isArray(loanDetails) &&
        loanDetails.length >= 2
      ) {
        const [loanAmount, loanToken] = loanDetails;
        console.log(loanAmount, loanToken);
        console.log(token);
        if (loanToken === token) {
          // Direct match with selected token
          repayableResult = Math.ceil(Number(loanAmount) * 1.05); // 5% interest
          console.log("Repayable result", repayableResult);
        }
      }

      // Update state with calculated values
      setBorrowableAmount(Math.floor(borrowableResult) || 0);
      setRepayableAmount(Math.floor(repayableResult) || 0);
    } catch (error) {
      console.error("Error fetching user amounts:", error);
      // Set default values in case of error
      setBorrowableAmount(0);
      setRepayableAmount(0);
    } finally {
      setIsLoadingAmounts(false);
    }
  }, [
    walletConnected,
    account,
    selectedToken,
    contractABI,
    publicClient,
    contractAddress,
    ETH,
    USDT,
  ]);
  // Note: removed publicClient from dependencies to prevent infinite loop

  // Fetch user's financial data when wallet connects or token changes
  useEffect(() => {
    if (walletConnected && account && selectedToken) {
      fetchUserAmounts();
    }
  }, [walletConnected, account, selectedToken, fetchUserAmounts]);

  /**
   * Handles tab switching between lend, borrow, and repay interfaces
   * @param {string} tab - The tab to switch to
   */
  const handleTabChange = (tab) => {
    setActiveTab(tab);
    setAmount(""); // Reset amount input when changing tabs
  };

  /**
   * Handles amount input changes, ensuring only integers are accepted
   * @param {Event} e - Input change event
   */
  const handleAmountChange = (e) => {
    // Remove any non-digit characters from the input value
    const value = e.target.value.replace(/[^\d]/g, "");

    // Update state with clean integer value
    setAmount(value);
  };

  /**
   * Handles token selection changes
   * @param {Event} e - Select change event
   */
  const handleTokenChange = (e) => {
    setSelectedToken(e.target.value);
    // This will trigger the useEffect to fetch updated amounts
  };

  /**
   * Handles the lending (deposit) functionality
   * Allows users to deposit ETH or USDT into the lending pool
   * @async
   * @returns {Promise<void>}
   */
  const handleLend = async () => {
    if (!walletConnected || !amount) return;
    setIsLoading(true); // Start loading indicator

    try {
      const token = selectedToken === "ETH" ? ETH : USDT;

      // Encode function call data for the deposit function
      const data = encodeFunctionData({
        abi: contractABI,
        functionName: "deposit",
      });

      console.log(amount);

      // Prepare transaction data including token amounts
      const txData = {
        to: contractAddress,
        data,
        tokens: [
          {
            id: token,
            amount: Number(amount),
          },
        ],
      };

      console.log("txData:", txData);

      // Send transaction using the =nil; wallet
      const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [txData],
      });

      console.log("Transaction hash:", txHash);
      setLastTxHash(txHash);

      // Wait for transaction to complete
      const receiptsAlternative = await waitTillCompleted(
        (await publicClient).publicClient,
        txHash,
      );

      console.log("Transaction Receipts:", receiptsAlternative);

      // Process transaction receipts to check for errors
      const error = processReceipts(receiptsAlternative);
      if (error) {
        console.log(`Transaction failed: ${error}`);
        alert(`Lending failed: ${error}`);
      } else {
        alert("Lending successful!");
        // Refresh amounts after successful transaction
        fetchUserAmounts();
      }

      setAmount(""); // Reset the amount field
    } catch (error) {
      console.error("Error in lending:", error);
      alert("Error in lending. Check console for details.");
    } finally {
      setIsLoading(false); // End loading indicator
    }
  };

  /**
   * Handles the borrowing functionality
   * Allows users to borrow against their deposited collateral
   * @async
   * @returns {Promise<void>}
   */
  const handleBorrow = async () => {
    if (!walletConnected || !amount) return;
    setIsLoading(true); // Start loading indicator

    try {
      const token = selectedToken === "ETH" ? ETH : USDT;

      // Encode function call data for the borrow function with arguments
      const data = encodeFunctionData({
        abi: contractABI,
        functionName: "borrow",
        args: [Number(amount), token],
      });

      console.log(amount);

      // Prepare transaction data
      const txData = {
        to: contractAddress,
        data,
      };

      console.log("txData:", txData);

      // Send transaction using the =nil; wallet
      const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [txData],
      });

      console.log("Transaction hash:", txHash);

      // Wait for transaction to complete
      const receiptsAlternative = await waitTillCompleted(
        (await publicClient).publicClient,
        txHash,
      );

      console.log(receiptsAlternative.length);
      console.log("Transaction Receipts:", receiptsAlternative);

      // Process transaction receipts to check for errors
      const error = processReceipts(receiptsAlternative);
      if (error) {
        console.log(`Transaction failed: ${error}`);
        alert(`Borrow failed: ${error}`);
      } else {
        alert("Borrowing successful!");
        // Refresh amounts after successful transaction
        fetchUserAmounts();
      }

      setLastTxHash(txHash);
      setAmount(""); // Reset the amount field
    } catch (error) {
      console.error("Error in borrowing:", error);
      alert("Error in borrowing. Check console for details.");
    } finally {
      setIsLoading(false); // End loading indicator
    }
  };

  /**
   * Handles the loan repayment functionality
   * Allows users to repay their outstanding loans with interest
   * @async
   * @returns {Promise<void>}
   */
  const handleRepay = async () => {
    if (!walletConnected || !amount) return;
    setIsLoading(true); // Start loading indicator

    try {
      const token = selectedToken === "ETH" ? ETH : USDT;

      // Encode function call data for the repayLoan function
      const data = encodeFunctionData({
        abi: contractABI,
        functionName: "repayLoan",
      });

      // Prepare transaction data including token amounts
      const txData = {
        to: contractAddress,
        data,
        tokens: [
          {
            id: token,
            amount: Number(amount),
          },
        ],
      };

      console.log("txData:", txData);

      // Send transaction using the =nil; wallet
      const txHash = await window.nil.request({
        method: "eth_sendTransaction",
        params: [txData],
      });

      console.log("Transaction hash:", txHash);

      // Wait for transaction to complete
      const receiptsAlternative = await waitTillCompleted(
        (await publicClient).publicClient,
        txHash,
      );

      console.log("Transaction Receipts:", receiptsAlternative);

      // Process transaction receipts to check for errors
      const error = processReceipts(receiptsAlternative);
      if (error) {
        console.log(`Transaction failed: ${error}`);
        alert(`Repay failed: ${error}`);
      } else {
        alert("Repay successful!");
        // Refresh amounts after successful transaction
        fetchUserAmounts();
      }

      setLastTxHash(txHash);
      setAmount(""); // Reset the amount field
    } catch (error) {
      console.error("Error in repaying:", error);
      alert("Error in repaying. Check console for details.");
    } finally {
      setIsLoading(false); // End loading indicator
    }
  };

  return (
    <div className="app">
      {/* Header with logo and wallet connection */}
      <header className="header">
        <div className="logo">DeFi Lending Platform</div>
        <div className="wallet-section">
          {!walletConnected ? (
            <button
              type="button"
              className="connect-wallet-btn"
              onClick={connectWallet}
            >
              Connect Wallet
            </button>
          ) : (
            <div className="account-info">
              {/* Display truncated wallet address */}
              {account.substring(0, 6)}...
              {account.substring(account.length - 4)}
            </div>
          )}
        </div>
      </header>

      <main className="main-content">
        {/* Introductory information */}
        <div className="info-text">
          <h2>Welcome to the Example Lending Protocol</h2>
          <p>
            This is an example application that lets you lend, borrow, and repay
            borrowed loans.
          </p>
          <p>
            <strong>Note:</strong> Make sure to install the{" "}
            <a
              href="https://chromewebstore.google.com/detail/nil-wallet/kfiailmjchdbjmadbkkldiahpggcjffp?hl=en-GB&utm_source=ext_sidebar"
              target="_blank"
              rel="noopener noreferrer"
            >
              =nil; wallet
            </a>{" "}
            to get started.
          </p>
          <p> This app is for demonstration purposes only.</p>
        </div>

        {/* Main application container */}
        <div className="container">
          {/* Tab navigation */}
          <div className="tabs">
            <button
              type="button"
              className={`tab ${activeTab === "lend" ? "active" : ""}`}
              onClick={() => handleTabChange("lend")}
              disabled={isLoading}
            >
              Lend
            </button>
            <button
              type="button"
              className={`tab ${activeTab === "borrow" ? "active" : ""}`}
              onClick={() => handleTabChange("borrow")}
              disabled={isLoading}
            >
              Borrow
            </button>
            <button
              type="button"
              className={`tab ${activeTab === "repay" ? "active" : ""}`}
              onClick={() => handleTabChange("repay")}
              disabled={isLoading}
            >
              Repay
            </button>
          </div>

          {/* Tab content area */}
          <div className="tab-content">
            {/* Lending tab panel */}
            {activeTab === "lend" && (
              <div className="tab-panel">
                <h3>Lend Assets</h3>
                <div className="input-container">
                  <input
                    type="text"
                    inputMode="numeric"
                    pattern="[0-9]*"
                    value={amount}
                    onChange={handleAmountChange}
                    placeholder="Enter amount"
                    disabled={isLoading}
                    onKeyPress={(e) => {
                      // Allow only digits (0-9)
                      if (!/[0-9]/.test(e.key)) {
                        e.preventDefault();
                      }
                    }}
                  />
                  <select
                    value={selectedToken}
                    onChange={handleTokenChange}
                    disabled={isLoading}
                  >
                    <option value="ETH">ETH</option>
                    <option value="USDT">USDT</option>
                  </select>
                </div>
                <p className="info-text-small">Deposit ETH or USDT to borrow</p>
                <button
                  type="button"
                  className="action-button"
                  onClick={handleLend}
                  disabled={isLoading || !amount}
                >
                  {isLoading ? "Processing..." : "Lend"}
                </button>
              </div>
            )}

            {/* Borrowing tab panel */}
            {activeTab === "borrow" && (
              <div className="tab-panel">
                <h3>Borrow Assets</h3>
                <div className="input-container">
                  <input
                    type="text"
                    inputMode="numeric"
                    pattern="[0-9]*"
                    value={amount}
                    onChange={handleAmountChange}
                    placeholder="Enter amount"
                    disabled={isLoading}
                    onKeyPress={(e) => {
                      // Allow only digits (0-9)
                      if (!/[0-9]/.test(e.key)) {
                        e.preventDefault();
                      }
                    }}
                  />
                  <select
                    value={selectedToken}
                    onChange={handleTokenChange}
                    disabled={isLoading}
                  >
                    <option value="ETH">ETH</option>
                    <option value="USDT">USDT</option>
                  </select>
                </div>
                {/* Display borrowable amount information */}
                <p className="info-text-small">
                  {isLoadingAmounts
                    ? "Loading borrowable amount..."
                    : walletConnected
                      ? `You can borrow up to ${borrowableAmount} ${selectedToken}`
                      : "Connect your wallet to see borrowable amount"}
                </p>
                <button
                  type="button"
                  className="action-button"
                  onClick={handleBorrow}
                  disabled={isLoading || !amount}
                >
                  {isLoading ? "Processing..." : "Borrow"}
                </button>
              </div>
            )}

            {/* Repayment tab panel */}
            {activeTab === "repay" && (
              <div className="tab-panel">
                <h3>Repay Loan</h3>
                <div className="input-container">
                  <input
                    type="text"
                    inputMode="numeric"
                    pattern="[0-9]*"
                    value={amount}
                    onChange={handleAmountChange}
                    placeholder="Enter amount"
                    disabled={isLoading}
                    onKeyPress={(e) => {
                      // Allow only digits (0-9)
                      if (!/[0-9]/.test(e.key)) {
                        e.preventDefault();
                      }
                    }}
                  />
                  <select
                    value={selectedToken}
                    onChange={handleTokenChange}
                    disabled={isLoading}
                  >
                    <option value="ETH">ETH</option>
                    <option value="USDT">USDT</option>
                  </select>
                </div>
                {/* Display repayable amount information */}
                <p className="info-text-small">
                  {isLoadingAmounts
                    ? "Loading repayable amount..."
                    : walletConnected
                      ? `You need to repay ${repayableAmount} ${selectedToken}`
                      : "Connect your wallet to see repayable amount"}
                </p>
                <button
                  type="button"
                  className="action-button"
                  onClick={handleRepay}
                  disabled={isLoading || !amount}
                >
                  {isLoading ? "Processing..." : "Repay"}
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Footer information section */}
        <div className="info-text transaction-note">
          <p className="exchange-rate">
            <strong>Exchange Rate:</strong> 1 ETH = 2 USDT
          </p>
          {/* Display transaction hash if available */}
          {lastTxHash && (
            <p className="tx-hash">
              <strong>Tx Hash:</strong> {lastTxHash}
            </p>
          )}
        </div>
      </main>
    </div>
  );
}

export default App;
