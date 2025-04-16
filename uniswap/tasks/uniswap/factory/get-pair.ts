import type { Address } from "abitype";
import { task } from "hardhat/config";

task("get-pair", "Retrieve the pair address for the specified tokens")
  .addParam("factory", "The address of the Uniswap V2 factory")
  .addParam("token0", "The address of the first token")
  .addParam("token1", "The address of the second token")
  .setAction(async (taskArgs, hre) => {
    const factoryAddress = taskArgs.factory as Address;
    const token0Address = taskArgs.token0 as Address;
    const token1Address = taskArgs.token1 as Address;

    const factory = await hre.nil.getContractAt("UniswapV2Factory", factoryAddress, {});

    const pairAddress = await factory.read.getTokenPair([
      token0Address,
      token1Address,
    ]);

    console.log(`Pair address: ${pairAddress}`);
  });
