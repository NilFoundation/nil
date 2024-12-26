import { getContract } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";
import FactoryJson from "../../../artifacts/contracts/UniswapV2Factory.sol/UniswapV2Factory.json";
import { createWallet } from "../../basic/basic";

task("get-pair", "Retrieve the pair address for the specified currencies")
  .addParam("factory", "The address of the Uniswap V2 factory")
  .addParam("currency0", "The address of the first currency")
  .addParam("currency1", "The address of the second currency")
  .setAction(async (taskArgs, _) => {
    // Destructure parameters for clarity
    const factoryAddress = taskArgs.factory as Address;
    const currency0Address = taskArgs.currency0 as Address;
    const currency1Address = taskArgs.currency1 as Address;

    const wallet = await createWallet();
    const factory = getContract({
      abi: FactoryJson.abi,
      address: factoryAddress,
      client: wallet.client,
      wallet: wallet,
    });

    // Retrieve the pair address
    const pairAddress = await factory.read.getTokenPair([
      currency0Address,
      currency1Address,
    ]);

    // Log the pair address
    console.log(`Pair address: ${pairAddress}`);
  });
