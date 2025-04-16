import { task } from "hardhat/config";

task("deploy-factory").setAction(async (taskArgs, hre) => {

  const smartAccount = await hre.nil.getSmartAccount();

  const { contract, address } = await hre.nil.deployContract("UniswapV2Factory", [smartAccount.address], {});
  console.log("Uniswap factory contract deployed at address: " + address);
});
