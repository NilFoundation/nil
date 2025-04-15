import { task } from "hardhat/config";

task("deploy-token")
  .addParam("amount")
  .setAction(async (taskArgs, hre) => {
    console.log("Deploying token contract...");

    const token = await hre.nil.deployContract("Token", [
      "USDT", BigInt(taskArgs.amount),
    ]);
    console.log("Deployed Token contract at address: ", token.address);
  });
