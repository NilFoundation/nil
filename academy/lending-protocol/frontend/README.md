# DeFi Lending Platform - Example Application

This is an **example application** designed for **educational purposes** to help developers learn how to interact with the =nil; blockchain using a frontend application. The app demonstrates the process of transacting with =nil;, including transacting with methods and tokens. It also showcases best practices for connecting a frontend application to the blockchain and calling smart contract methods, including handling tokens.

## Purpose

The goal of this application is to give developers an understanding of how to build frontend applications that interact with =nil; and its smart contracts. By going through this example, you can learn the following:

- How to **connect to the =nil; wallet**.
- How to **read data** from a smart contract.
- How to **call smart contract methods** for transactions such as lending, borrowing, and repaying, including sending tokens along with method calls via the =nil; wallet.
- How to structure your frontend code to handle receipts and errors.

### Features

This application allows you to:

- Depositing assets (ETH or USDT) into the protocol
- Borrowing assets up to 80% of your deposit value
- Repaying loans with 5% interest (105% of the borrowed amount)
- Viewing real-time deposit and loan balances

> Note: This example application only supports whole numbers (no decimals) for all transactions.

This repository is designed to provide a simple yet functional template for building decentralized applications (dApps) on top of =nil;.

---

## Pre-requisites

Before running this application, ensure that you have the following:

- **=nil; wallet extension** installed on your browser. You can get it from the [Chrome Web Store](https://chromewebstore.google.com/detail/nil-wallet/kfiailmjchdbjmadbkkldiahpggcjffp?hl=en-GB&utm_source=ext_sidebar).
- **Node.js** and **npm** installed on your machine.
- A basic understanding of React, TypeScript, and how smart contracts work.

---

## Installation

### 1. Clone the repository

First, clone this repository to your local machine:

```bash
git clone https://github.com/NilFoundation/nil
cd  nil/academy/lending-protocol/frontend
```

### 2. Install dependencies

Install the necessary dependencies using npm:

```bash
npm install --legacy-peer-deps
```

### 3. Configure the RPC URL

Before running the frontend application, you must deploy the lending pool contract and obtain its address.

<strong>Deploy the Lending Pool Contract</strong>

If you haven’t already deployed the contract, navigate to the lending protocol directory and deploy it using Hardhat:

```bash
cd ../ # Move to the lending protocol directory
npx hardhat compile
npx hardhat run-lending-protocol
```

Once the deployment completes, look for a log similar to:

`Lending Pool deployed at: 0xYourContractAddressHere`

Copy the contract address.

<strong>Configure the Frontend with the Contract Address</strong>

You need to specify the RPC URL for your =nil; blockchain in the `constants.js` file, which is used to connect to the blockchain network.

In `src/constants.js`, update the following line with your RPC URL:

```javascript
const NIL_RPC_URL = "<YOUR_RPC_URL>";
const contractAddress = "your_contract_address";
```

### 4. Run the Application

Once the installation is complete, start the development server with:

```bash
npm run start
```

This will launch the application, and you can access it via `http://localhost:3000/` in your web browser.

---

## Coding Patterns and Best Practices

This section will walk you through the key coding patterns used in this app. These patterns will help you interact with the =nil; blockchain and smart contracts.

### 1. **Connecting to the Wallet**

To interact with =nil; via a frontend, the first step is to connect to the user's wallet. The app uses `eth_requestAccounts` to prompt the user to connect their =nil; wallet.

```javascript
const connectWallet = async () => {
  try {
    if (window.nil) {
      const accounts = await window.nil.request({
        method: "eth_requestAccounts",
      });
      setAccount(accounts[0]);
      setWalletConnected(true);
    } else {
      alert("Please install =nil; wallet from the chrome store!");
    }
  } catch (error) {
    console.error("Error connecting wallet:", error);
  }
};
```

---

### 2. **Reading Data from a Smart Contract**

To interact with a smart contract, we use the `getContract` method from `niljs`. This allows us to call both **view functions** (read operations) and **write functions** (state-changing operations).

In this particular case, we use it to read transaction data from the contract.

```javascript
const fetchUserAmounts = async () => {
  if (!walletConnected || !account) return;

  setIsLoadingAmounts(true);
  try {
    const client = await publicClient.publicClient;
    const contract = await getContract({
      abi: contractABI,
      address: contractAddress,
      client,
    });

    const globalLedger = await contract.read.globalLedger();

    const globalLedgerContract = await getContract({
      abi: globalLedgerAbi.abi,
      address: globalLedger,
      client,
    });

    const ethDepositAmount = await globalLedgerContract.read.getDeposit([
      account,
      ETH,
    ]);
    const usdtDepositAmount = await globalLedgerContract.read.getDeposit([
      account,
      USDT,
    ]);

    console.log("ETH deposit:", ethDepositAmount);
    console.log("USDT deposit:", usdtDepositAmount);
  } catch (error) {
    console.error("Error fetching user amounts:", error);
  } finally {
    setIsLoadingAmounts(false);
  }
};
```

If you want to **write to a contract**, please refer to [this example](https://github.com/NilFoundation/nil/blob/main/uniswap/tasks/uniswap/demo-router.ts#L81).

---

### 3. **Calling a Contract Method with and Without Tokens**

There are two ways to interact with smart contracts: calling a method **without** tokens (e.g., borrowing) and calling a method **with** tokens (e.g., deposit or repay).

#### Example: Calling a Method Without Tokens (Borrowing)

```javascript
const handleBorrow = async () => {
  if (!walletConnected || !amount) return;

  try {
    const token = selectedToken === "ETH" ? ETH : USDT;
    const data = encodeFunctionData({
      abi: contractABI,
      functionName: "borrow",
      args: [Number(amount), token],
    });

    const txData = {
      to: contractAddress,
      data,
    };

    await window.nil.request({
      method: "eth_sendTransaction",
      params: [txData],
    });
  } catch (error) {
    console.error("Error in borrowing:", error);
  }
};
```

#### Example: Calling a Method With Tokens (Deposit or Repay)

```javascript
const handleDeposit = async () => {
  if (!walletConnected || !amount) return;

  try {
    const token = selectedToken === "ETH" ? ETH : USDT;
    const data = encodeFunctionData({
      abi: contractABI,
      functionName: "deposit",
    });

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

    await window.nil.request({
      method: "eth_sendTransaction",
      params: [txData],
    });
  } catch (error) {
    console.error("Error in depositing:", error);
  }
};
```

---

### 4. **Error Handling in Transactions**

Handling errors in blockchain transactions is crucial. This application shows how you can processes transaction receipts and extracts error messages when transactions fail.

Example: Processing Errors from Receipts

```javascript
export function processReceipts(receipts) {
  for (let i = 0; i < receipts.length; i++) {
    const receipt = receipts[i];

    // If the transaction is not successful, return the error message or status
    if (!receipt.success || receipt.status !== "Success") {
      const errorMessage = receipt.errorMessage || receipt.status;
      return errorMessage; // Exit the loop and return the first error message encountered
    }
  }

  // If all transactions are successful, return null
  return null;
}
```

This function is used after transactions are completed to check if any errors occurred.

```javascript
const receipts = await waitTillCompleted(client, txHash);
const error = processReceipts(receipts);
if (error) {
  console.log(`Transaction failed: ${error}`);
  alert(`Transaction failed: ${error}`);
} else {
  alert("Transaction successful!");
}
```
This ensures that failed transactions return clear error messages to the user.

## Contribution Guidelines

1. **Fork** the repository.
2. Check for open issues [here](https://github.com/NilFoundation/nil/issues).
3. Read the [Contribution Guide](https://github.com/NilFoundation/nil/blob/main/CONTRIBUTION-GUIDE.md).
4. **Submit** a pull request with a detailed description of your changes based on the Contribution Guide.

Feel free to contribute and report any issues you encounter. Happy coding!
