const { task } = require("hardhat/config");

task("deploy-lending-pool", "Deploys a new LendingPool contract and registers it in GlobalLedger")
  .addParam("factory", "The address of the LendingPoolFactory contract")
  .addParam("globalLedger", "The address of the GlobalLedger contract")
  .addParam("interestManager", "The address of the InterestManager contract")
  .addParam("oracle", "The address of the Oracle contract")
  .addParam("usdt", "The TokenId for USDT")
  .addParam("eth", "The TokenId for ETH")
  .setAction(async (taskArgs) => {
    const { ethers } = require("hardhat");

    const factoryAddress = taskArgs.factory;
    const globalLedgerAddress = taskArgs.globalLedger;
    const interestManagerAddress = taskArgs.interestManager;
    const oracleAddress = taskArgs.oracle;
    const usdtTokenId = taskArgs.usdt;
    const ethTokenId = taskArgs.eth;

    console.log("Deploying LendingPool using the provided factory...");

    // Attach to LendingPoolFactory contract
    const LendingPoolFactory = await ethers.getContractFactory("LendingPoolFactory");
    const factory = LendingPoolFactory.attach(factoryAddress);

    // Deploy LendingPool via factory
    const tx = await factory.deployLendingPool();
    await tx.wait(); // Wait for the transaction to be mined
    console.log("LendingPool deployed successfully!");

    // Interact with GlobalLedger to check if the LendingPool is registered
    const GlobalLedger = await ethers.getContractFactory("GlobalLedger");
    const globalLedger = GlobalLedger.attach(globalLedgerAddress);

    // Check if LendingPool is registered in GlobalLedger
    const isRegistered = await globalLedger.authorizedLendingPools(factoryAddress);
    console.log("Is LendingPool registered in GlobalLedger?", isRegistered);

    // You can also test other functionality like registering a deposit, loan, etc.
  });

module.exports = {};