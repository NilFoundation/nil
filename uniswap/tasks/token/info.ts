import { generateRandomPrivateKey, getContract, LocalECDSAKeySigner } from "@nilfoundation/niljs";
import { task } from "hardhat/config";
import { createSmartAccount } from "../basic/basic";

task("token-info", "Retrieve token name and ID")
  .addParam("address", "The address of the deployed token contract")
  .setAction(async (taskArgs, hre) => {
    const smartAccount = await createSmartAccount();

    const contract = await hre.nil.getContractAt("Token", taskArgs.address, {
      publicClient: smartAccount.client,
      smartAccount: smartAccount,
    });

    // Retrieve the token's name
    const tokenName = await contract.read.getTokenName([]);
    console.log("Token Name: " + tokenName);

    // Retrieve the token's unique ID
    const tokenId = await contract.read.getTokenId([]);
    console.log("Token ID: " + tokenId);

    // Retrieve the contract's own token balance
    const balance = await contract.read.getOwnTokenBalance([]);
    const balance2 = await contract.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    console.log("Token Balance: " + balance + " " + balance2);
  });
