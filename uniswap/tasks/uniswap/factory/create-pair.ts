import { getContract, waitTillCompleted } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";
import FactoryJson from "../../../artifacts/contracts/UniswapV2Factory.sol/UniswapV2Factory.json";
import PairJson from "../../../artifacts/contracts/UniswapV2Pair.sol/UniswapV2Pair.json";
import { createWallet } from "../../basic/basic";

task("create-pair", "Deploy and initialize a new Uniswap V2 pair")
  .addParam("factory", "The address of the Uniswap V2 factory")
  .addParam("currency0", "The address of the first currency")
  .addParam("currency1", "The address of the second currency")
  .setAction(async (taskArgs, _) => {
    // Destructure parameters for clarity
    const factoryAddress = taskArgs.factory as Address;
    const currency0Address = taskArgs.currency0 as Address;
    const currency1Address = taskArgs.currency1 as Address;
    const shardId = 1;

    const wallet = await createWallet();

    const factory = getContract({
      abi: FactoryJson.abi,
      address: factoryAddress,
      client: wallet.client,
      wallet: wallet,
    });

    // Create the pair and get its address
    const createPairTx = await factory.write.createPair([
      currency0Address,
      currency1Address,
      Math.floor(Math.random() * 10000000),
      shardId,
    ]);
    await waitTillCompleted(wallet.client, createPairTx);

    const pairAddress = await factory.read.getTokenPair([
      currency0Address,
      currency1Address,
    ]);

    // Log the pair address
    console.log(`Pair created successfully at address: ${pairAddress}`);

    // Attach to the newly created Uniswap V2 Pair contract
    const pair = getContract({
      abi: PairJson.abi,
      address: pairAddress as Address,
      client: wallet.client,
      wallet: wallet,
    });

    // Initialize the pair with currency addresses and IDs
    const hash = await pair.write.initialize([
      currency0Address,
      currency1Address,
    ]);
    await waitTillCompleted(wallet.client, hash);

    console.log(`Pair initialized successfully at address: ${pairAddress}`);
  });
