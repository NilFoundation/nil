import { ethers } from "ethers";
import fs from "fs";
import path from "path";
import dotenv from "dotenv";
dotenv.config();

async function main() {
  // Connect to the local Geth node
  const L1_RPC_ENDPOINT = process.env.L1_RPC_ENDPOINT as string;
  console.log("L1 RPC Endpoint:", L1_RPC_ENDPOINT);
  const provider = new ethers.JsonRpcProvider(L1_RPC_ENDPOINT);

  // Predefined private key from environment variable
  const privateKey = process.env.PRIVATE_KEY;
  if (!privateKey) {
    throw new Error("PRIVATE_KEY environment variable is not set");
  }

  const accounts = await provider.send("eth_accounts", []);
  const defaultAccount = accounts[0];
  console.log("Default Account Address:", defaultAccount);

  console.log("Wallet Connected to Provider.");

  const valueInHex = ethers.hexlify(ethers.parseEther("100"));
  const walletAddress = process.env.WALLET_ADDRESS as string;
  console.log("Wallet Address:", walletAddress);

  const fundingTx = await provider.send("eth_sendTransaction", [
      {
          from: defaultAccount,
          to: walletAddress,
          value: valueInHex,
      },
  ]);

  console.log("Funding Transaction Sent:", fundingTx);

  const fundTransactionHash = fundingTx;
  console.log("Funding Transaction Hash:", fundTransactionHash);

  // Wait for the transaction to be mined
  const fundReceipt = await provider.waitForTransaction(fundTransactionHash);
  console.log("Funding Transaction Mined:", fundReceipt);

  // query balance
  const balance = await provider.getBalance(walletAddress);
  console.log(`Balance of Wallet: ${walletAddress} After funding:" ${balance}`);

  // Read the compiled contract ABI and bytecode
  const abi = JSON.parse(fs.readFileSync(path.join(__dirname, "NilChain.abi"), "utf8"));
  const bytecode = fs.readFileSync(path.join(__dirname, "NilChain.bin"), "utf8");

  const wallet = new ethers.Wallet(privateKey, provider);

  // Create a ContractFactory and deploy the contract
  const factory = new ethers.ContractFactory(abi, bytecode, wallet);

  const chainId = parseInt(process.env.CHAIN_ID as string, 10);
  const version = parseInt(process.env.VERSION as string, 10);

  const contract = await factory.deploy(chainId, version);
  const deployReceipt = await contract.waitForDeployment();

  if (!deployReceipt || deployReceipt.getAddress() === null) {
    throw new Error("NilChain Contract deployment failed");
  }

  const contractAddress = await contract.getAddress();
  console.log("NilChain contract deployed at:", contractAddress);

  // Wait for a few seconds
  await new Promise((resolve) => setTimeout(resolve, 5000));

  // Create a contract instance with the correct ABI and address
  const nilChainContract = new ethers.Contract(contractAddress, abi, wallet);

  // Call setSyncCommMemberStatus with the deployer address and _status set to true
  const setSyncCommMemberStatusTxn = await nilChainContract.setSyncCommMemberStatus(walletAddress, true);
  await setSyncCommMemberStatusTxn.wait();

  console.log(`setSyncCommMemberStatus called with member: ${walletAddress}, status: true`);

  // Query if the member is really added to the sync committee
  const isMember = await nilChainContract.isCommitteeMember(walletAddress);
  console.log(`Is ${walletAddress} a committee member?`, isMember);  
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
