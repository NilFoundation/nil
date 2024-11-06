import { task } from "hardhat/config";
import type {
  Currency,
  UniswapV2Factory,
  UniswapV2Pair,
} from "../../../typechain-types";

task("create-pair", "Deploy and initialize a new Uniswap V2 pair")
  .addParam("factory", "The address of the Uniswap V2 factory")
  .addParam("currency0", "The address of the first currency")
  .addParam("currency1", "The address of the second currency")
  .setAction(async (taskArgs, hre) => {
    // Destructure parameters for clarity
    const factoryAddress = taskArgs.factory;
    const currency0Address = taskArgs.currency0;
    const currency1Address = taskArgs.currency1;
    const shardId = 1;

    // Attach to the Uniswap V2 Factory contract
    const factoryContract =
      await hre.ethers.getContractFactory("UniswapV2Factory");
    const factory = factoryContract.attach(factoryAddress) as UniswapV2Factory;

    // Create the pair and get its address
    await factory.createPair(
      currency0Address.toLowerCase(),
      currency1Address.toLowerCase(),
      Math.floor(Math.random() * 10000000),
      shardId,
    );

    const pairAddress = await factory.getTokenPair(
      currency0Address.toLowerCase(),
      currency1Address.toLowerCase(),
    );

    // Log the pair address
    console.log(`Pair created successfully at address: ${pairAddress}`);

    // Attach to the Currency contract for both currencies
    const currencyContract = await hre.ethers.getContractFactory("Currency");

    const firstCurrency = currencyContract.attach(currency0Address) as Currency;
    const firstCurrencyId = await firstCurrency.getCurrencyId();
    console.log(`First currency ID: ${firstCurrencyId}`);

    const secondCurrency = currencyContract.attach(
      currency1Address,
    ) as Currency;
    const secondCurrencyId = await secondCurrency.getCurrencyId();
    console.log(`Second currency ID: ${secondCurrencyId}`);

    // Attach to the newly created Uniswap V2 Pair contract
    const pairContract = await hre.ethers.getContractFactory("UniswapV2Pair");
    const pair = pairContract.attach(pairAddress) as UniswapV2Pair;

    // Initialize the pair with currency addresses and IDs
    await pair.initialize(
      currency0Address,
      currency1Address,
      firstCurrencyId,
      secondCurrencyId,
    );

    console.log(`Pair initialized successfully at address: ${pairAddress}`);
  });
