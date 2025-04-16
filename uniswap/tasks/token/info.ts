import { task } from "hardhat/config";

task("token-info", "Retrieve token name and balance")
  .addParam("address", "The address of the deployed token contract")
  .setAction(async (taskArgs, hre) => {

    const contract = await hre.nil.getContractAt("Token", taskArgs.address);

    // Retrieve the token's name
    const tokenName = await contract.read.getTokenName([]);
    console.log("Token Name: " + tokenName);

    // Retrieve the contract's own token balance
    const balance = await contract.read.getOwnTokenBalance([]);
    console.log("Token Balance: " + balance);
  });
