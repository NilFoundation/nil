import { Wallet, ethers } from "ethers";
import * as dotenv from "dotenv";
dotenv.config();

// npx ts-node scripts/create-wallet-with-funding.ts
async function createAndUseWallet() {
  const provider = new ethers.JsonRpcProvider("http://localhost:8545"); // Change URL as needed

  const accounts = await provider.send("eth_accounts", []);
  const defaultAccount = accounts[0];
  console.log("Default Account Address:", defaultAccount);

  console.log("Wallet Connected to Provider.");

  const wallet = Wallet.createRandom();
  console.log("New Wallet Created:");
  console.log("Address:", wallet.address);
  console.log("Private Key:", wallet.privateKey);
  const connectedWallet = wallet.connect(provider);

  const value = ethers.parseEther("1");
  console.log("Value:", value.toString());
  const valueInHex = ethers.toQuantity(ethers.parseEther("1"));
  console.log("Value in Hex:", valueInHex);

  const fundingTx = await provider.send("eth_sendTransaction", [
    {
      from: defaultAccount,
      to: wallet.address,
      value: valueInHex,
    },
  ]);

  console.log("Funding Transaction Sent:", fundingTx);

  // Step 1: Test Create a new random wallet
  const receivingWallet = Wallet.createRandom();

  // Step 2: Display wallet details
  console.log("New Wallet Created:");
  console.log("Address:", receivingWallet.address);
  console.log("Private Key:", receivingWallet.privateKey);

  // Step 3: Use the wallet to send a transaction
  const tx = await connectedWallet.sendTransaction({
    to: receivingWallet.address,
    value: ethers.parseEther("0.1"),
    gasLimit: 21000,
  });

  console.log("Transaction Sent:", tx.hash);

  // Step 4: Wait for the transaction to be mined
  const receipt = await tx.wait();
  console.log("Transaction Mined:", receipt?.hash);
}

createAndUseWallet().catch((error) => {
  console.error("Error:", error.message);
});
