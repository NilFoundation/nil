import { waitTillCompleted } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";

task("create-pair", "Deploy and initialize a new Uniswap V2 pair")
  .addParam("factory", "The address of the Uniswap V2 factory")
  .addParam("token0", "The address of the first token")
  .addParam("token1", "The address of the second token")
  .setAction(async (taskArgs, hre) => {
    // Destructure parameters for clarity
    const factoryAddress = taskArgs.factory as Address;
    const token0Address = taskArgs.token0 as Address;
    const token1Address = taskArgs.token1 as Address;
    const shardId = 1;

    const client = await hre.nil.getPublicClient();

    const factory = await hre.nil.getContractAt("UniswapV2Factory", factoryAddress, {});

    // Create the pair and get its address
    const createPairTx = await factory.write.createPair([
      token0Address,
      token1Address,
      BigInt(Math.floor(Math.random() * 10000000)),
      BigInt(shardId),
    ]);
    await waitTillCompleted(client, createPairTx);

    const pairAddress = await factory.read.getTokenPair([
      token0Address,
      token1Address,
    ]);

    console.log(`Pair created successfully at address: ${pairAddress}`);

    // Attach to the newly created Uniswap V2 Pair contract
    const pair = await hre.nil.getContractAt("UniswapV2Pair", pairAddress, {});

    // Initialize the pair with token addresses and IDs
    const hash = await pair.write.initialize([token0Address, token1Address]);
    await waitTillCompleted(client, hash);

    console.log(`Pair initialized successfully at address: ${pairAddress}`);
  });
