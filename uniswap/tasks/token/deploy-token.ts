import { task } from "hardhat/config";

task("deploy-token")
  .addParam("amount")
  .setAction(async (taskArgs, hre) => {
    const token = await hre.nil.deployContract("Token", [
      "USDT", BigInt(taskArgs.amount),
    ]);
  });
