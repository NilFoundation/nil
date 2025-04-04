const { task } = require("hardhat/config");
const { Nil } = require("niljs");

task("deploy-lending-pool", "Deploys a new LendingPool and registers in GlobalLedger")
  .addParam("factory", "LendingPoolFactory contract address")
  .addParam("globalledger", "GlobalLedger contract address")
  .addParam("interestmanager", "InterestManager contract address")
  .addParam("oracle", "Oracle address")
  .addParam("usdt", "USDT TokenId")
  .addParam("eth", "ETH TokenId")
  .setAction(async (taskArgs, hre) => {
    const {
      factory,
      globalledger,
      interestmanager,
      oracle,
      usdt,
      eth,
    } = taskArgs;

    const nil = new Nil(hre.network.config.nilProviderUrl); // configure `nilProviderUrl` in hardhat.config.ts

    console.log("Deploying LendingPool via asyncDeploy...");

    const deployTx = await nil.asyncDeploy({
      to: factory,
      value: "0", // if needed
      input: hre.ethers.utils.defaultAbiCoder.encode(
        ["address", "address", "address", "TokenId", "TokenId", "address"],
        [globalledger, interestmanager, oracle, usdt, eth, hre.ethers.provider.getSigner().getAddress()]
      ),
    });

    console.log("LendingPool deployment asyncDeploy Tx:", deployTx.hash);

    // You can optionally wait for confirmation
    await deployTx.wait();

    console.log("LendingPool deployed and asyncCall to GlobalLedger initiated.");
  });