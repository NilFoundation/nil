import { task } from "hardhat/config";
import { createWallet } from "./basic";

task("create-wallet").setAction(async (taskArgs, _) => {
  const wallet = await createWallet({
    faucetDeposit: true,
  });
  console.log("Wallet created: " + wallet.address);
});
