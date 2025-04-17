import { scope } from "hardhat/config";
import type { HardhatRuntimeEnvironment } from "hardhat/types";

const walletTask = scope("wallet", "Wallet tasks");

walletTask
  .task("info", "Print info about current wallet")
  .setAction(async (taskArgs, hre: HardhatRuntimeEnvironment) => {
    const smartAccount = await hre.nil.getSmartAccount()

    console.log("Current wallet:");
    console.log(`  address: ${smartAccount.address}`);
  });

walletTask
  .task("create")
  .setAction(async (taskArgs, hre) => {
    const smartAccount = await hre.nil.createSmartAccount({});
    const balance = await smartAccount.getBalance();
    console.log("Smart account created: " + smartAccount.address);
    console.log("Smart account balance: " + balance);
  });
