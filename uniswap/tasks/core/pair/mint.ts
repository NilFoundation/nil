import { shardNumber } from "@nilfoundation/hardhat-plugin/dist/utils/conversion";
import { waitTillCompleted } from "@nilfoundation/niljs";
import { task } from "hardhat/config";
import { encodeFunctionData } from "viem";
import type { Currency, UniswapV2Pair } from "../../../typechain-types";
import { createClient } from "../../util/client";

task("mint", "Mint currencies and add liquidity to the pair")
  .addParam("pair", "The address of the pair contract")
  .addParam("amount0", "The amount of the first currency to mint")
  .addParam("amount1", "The amount of the second currency to mint")
  .setAction(async (taskArgs, hre) => {
    const walletAddress = process.env.WALLET_ADDR;

    if (!walletAddress) {
      throw new Error("WALLET_ADDR is not set in environment variables");
    }

    const pairArtifact = await hre.artifacts.readArtifact("UniswapV2Pair");

    const { wallet, publicClient } = await createClient();

    // Destructure parameters for clarity
    const pairAddress = taskArgs.pair;
    const amount0 = taskArgs.amount0;
    const amount1 = taskArgs.amount1;

    // Attach to the Uniswap V2 Pair contract
    const PairFactory = await hre.ethers.getContractFactory("UniswapV2Pair");
    const pair = PairFactory.attach(pairAddress) as UniswapV2Pair;

    // Fetch currency addresses from the pair contract
    const currency0Address = await pair.token0();
    const currency1Address = await pair.token1();

    console.log("Currency 0 Address:", currency0Address.toString());
    console.log("Currency 1 Address:", currency1Address.toString());

    // Attach to the Currency contracts
    const CurrencyFactory = await hre.ethers.getContractFactory("Currency");
    const currency0 = CurrencyFactory.attach(currency0Address) as Currency;
    const currency1 = CurrencyFactory.attach(currency1Address) as Currency;

    // Retrieve and log currency IDs
    const currency0Id = await currency0.getCurrencyId();
    console.log("Currency 0 ID:", currency0Id);

    const currency1Id = await currency1.getCurrencyId();
    console.log("Currency 1 ID:", currency1Id);

    // Send currency amounts to the pair contract
    console.log(
      `Sending ${amount0} currency0 and ${amount1} currency1 to ${pairAddress}...`,
    );

    // Mint liquidity
    console.log("Minting pair tokens...");

    const hash = await wallet.sendMessage({
      to: pairAddress,
      feeCredit: BigInt(10_000_000),
      value: BigInt(0),
      refundTo: wallet.address,
      data: encodeFunctionData({
        abi: pairArtifact.abi,
        functionName: "mint",
        args: [walletAddress],
      }),
      tokens: [
        {
          id: currency0Id,
          amount: BigInt(amount0),
        },
        {
          id: currency1Id,
          amount: BigInt(amount1),
        },
      ],
    });

    await waitTillCompleted(publicClient, shardNumber(walletAddress), hash);

    // Log balances in the pair contract
    const pairCurrency0Balance =
      await currency0.getCurrencyBalanceOf(pairAddress);
    console.log("Pair Balance 0:", pairCurrency0Balance.toString());

    const pairCurrency1Balance =
      await currency1.getCurrencyBalanceOf(pairAddress);
    console.log("Pair Balance 1:", pairCurrency1Balance.toString());

    console.log("Liquidity added...");

    // Retrieve and log reserves from the pair
    const [reserve0, reserve1] = await pair.getReserves();
    console.log(
      `Reserves - Currency0: ${reserve0.toString()}, Currency1: ${reserve1.toString()}`,
    );

    // Check and log liquidity provider balance
    const lpBalance = await pair.getCurrencyBalanceOf(walletAddress);
    console.log("Liquidity provider balance in wallet:", lpBalance.toString());

    // Retrieve and log total supply for the pair
    const totalSupply = await pair.getCurrencyTotalSupply();
    console.log("Total supply of pair tokens:", totalSupply.toString());
  });
