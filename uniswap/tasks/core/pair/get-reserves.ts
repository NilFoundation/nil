import { task } from "hardhat/config";
import type { UniswapV2Pair } from "../../../typechain-types";

task("get-reserves", "Retrieve reserves for the specified pair")
  .addParam("pair", "The address of the Uniswap V2 pair contract")
  .setAction(async (taskArgs, hre) => {
    // Destructure parameters for clarity
    const pairAddress = taskArgs.pair.toLowerCase();

    // Attach to the Uniswap V2 Pair contract
    const pairContract = await hre.ethers.getContractFactory("UniswapV2Pair");
    const pair = pairContract.attach(pairAddress) as UniswapV2Pair;

    // Retrieve reserves
    const [reserve0, reserve1] = await pair.getReserves();

    // Log the reserves
    console.log(
      `Reserves - Currency0: ${reserve0.toString()}, Currency1: ${reserve1.toString()}`,
    );
  });
