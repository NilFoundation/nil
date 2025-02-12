import { Wallet, ethers } from "ethers";
import * as dotenv from "dotenv";
dotenv.config();

// npx ts-node scripts/fund-wallet.ts
async function createAndUseWallet() {
  const GETH_RPC_ENDPOINT = process.env.GETH_RPC_ENDPOINT as string;
  console.log("L1 RPC Endpoint:", GETH_RPC_ENDPOINT);
  const provider = new ethers.JsonRpcProvider(GETH_RPC_ENDPOINT);

  const accounts = await provider.send("eth_accounts", []);
  const defaultAccount = accounts[0];
  console.log("Default Account Address:", defaultAccount);

  console.log("Wallet Connected to Provider.");

  const valueInHex = ethers.toQuantity(ethers.parseEther("100"));
  console.log("Value in Hex:", valueInHex);

  const walletAddress = process.env.GETH_WALLET_ADDRESS as string;
  console.log("Wallet Address:", walletAddress);

  const fundingTx = await provider.send("eth_sendTransaction", [
    {
      from: defaultAccount,
      to: walletAddress,
      value: valueInHex,
    },
  ]);

  console.log("Funding Transaction Sent:", fundingTx);

  const transactionHash = fundingTx;
  console.log("Transaction Hash:", transactionHash);

  // Wait for the transaction to be mined
  const receipt = await provider.waitForTransaction(transactionHash);
  console.log("Transaction Mined:", receipt);

  // query balance
  const balance = await provider.getBalance(walletAddress);
  console.log("Balance:", balance);
}

createAndUseWallet().catch((error) => {
  console.error("Error:", error.message);
});
