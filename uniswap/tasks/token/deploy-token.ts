import { task } from "hardhat/config";

task("deploy-token")
  .addParam("amount")
  .setAction(async (taskArgs, hre) => {
    console.log("Deploying token contract...");

    const newAccount = await hre.nil.createSmartAccount();
    const token = await hre.nil.deployContract("Token", []);
    // TODO: complete to hh plugin
    console.log("Deployed Token contract at address: ", token.address);
  });
