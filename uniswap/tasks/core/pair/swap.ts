import { shardNumber } from "@nilfoundation/hardhat-plugin/dist/utils/conversion";
import { waitTillCompleted } from "@nilfoundation/niljs";
import { task } from "hardhat/config";
import { encodeFunctionData } from "viem";
import type { Currency, UniswapV2Pair } from "../../../typechain-types";
import { createClient } from "../../util/client";

task("swap", "Swap currency0 for currency1 in the Uniswap pair")
  .addParam("pair", "The address of the Uniswap pair contract")
  .addParam("amount", "The amount of currency0 to swap")
  .setAction(async (taskArgs, hre) => {
    const walletAddress = process.env.WALLET_ADDR;

    if (!walletAddress) {
      throw new Error("WALLET_ADDR is not set in environment variables");
    }

    const pairArtifact = await hre.artifacts.readArtifact("UniswapV2Pair");

    const { wallet, publicClient } = await createClient();

    // Destructure parameters for clarity
    const pairAddress = taskArgs.pair.toLowerCase();
    const swapAmount = BigInt(taskArgs.amount);

    // Attach to the Uniswap V2 Pair contract
    const PairFactory = await hre.ethers.getContractFactory("UniswapV2Pair");
    const pair = PairFactory.attach(pairAddress) as UniswapV2Pair;

    // Retrieve currency addresses from the pair contract
    const currency0Address = await pair.token0();
    const currency1Address = await pair.token1();

    console.log("Currency 0 Address:", currency0Address.toString());
    console.log("Currency 1 Address:", currency1Address.toString());

    // Attach to the Currency contracts
    const CurrencyFactory = await hre.ethers.getContractFactory("Currency");
    const currency0Contract = CurrencyFactory.attach(
      currency0Address,
    ) as Currency;
    const currency1Contract = CurrencyFactory.attach(
      currency1Address,
    ) as Currency;

    // Retrieve currency IDs
    const currency0Id = await currency0Contract.getCurrencyId();
    console.log("Currency ID for currency0:", currency0Id);

    const currency1Id = await currency1Contract.getCurrencyId();
    console.log("Currency ID for currency1:", currency1Id);

    // Retrieve reserves from the pair
    const reserves = await pair.getReserves();
    const reserve0 = reserves[0];
    const reserve1 = reserves[1];
    console.log(`Reserves - Currency0: ${reserve0}, Currency1: ${reserve1}`);

    // Calculate expected output amount for the swap
    const expectedOutputAmount = calculateOutputAmount(
      swapAmount,
      reserve0,
      reserve1,
    );
    console.log(
      "Expected output amount for swap:",
      expectedOutputAmount.toString(),
    );

    // Log balances before the swap
    const balanceCurrency0Before =
      await currency0Contract.getCurrencyBalanceOf(walletAddress);
    const balanceCurrency1Before =
      await currency1Contract.getCurrencyBalanceOf(walletAddress);
    console.log(
      "Balance of currency0 before swap:",
      balanceCurrency0Before.toString(),
    );
    console.log(
      "Balance of currency1 before swap:",
      balanceCurrency1Before.toString(),
    );

    // Execute the swap
    console.log("Executing swap...");

    const hash = await wallet.sendMessage({
      // @ts-ignore
      to: pairAddress,
      feeCredit: BigInt(10_000_000),
      value: BigInt(0),
      data: encodeFunctionData({
        abi: pairArtifact.abi,
        functionName: "swap",
        args: [0, expectedOutputAmount, walletAddress],
      }),
      refundTo: wallet.address,
      tokens: [
        {
          id: currency0Id,
          amount: BigInt(swapAmount),
        },
      ],
    });

    await waitTillCompleted(publicClient, shardNumber(walletAddress), hash);

    console.log(
      `Sent ${swapAmount.toString()} of currency0 to the pair contract.`,
    );

    console.log("Swap executed successfully.");

    // Log balances after the swap
    const balanceCurrency0After =
      await currency0Contract.getCurrencyBalanceOf(walletAddress);
    const balanceCurrency1After =
      await currency1Contract.getCurrencyBalanceOf(walletAddress);
    console.log(
      "Balance of currency0 after swap:",
      balanceCurrency0After.toString(),
    );
    console.log(
      "Balance of currency1 after swap:",
      balanceCurrency1After.toString(),
    );
  });

// Function to calculate the output amount for the swap
function calculateOutputAmount(
  amountIn: bigint,
  reserveIn: bigint,
  reserveOut: bigint,
): bigint {
  const amountInWithFee = amountIn * BigInt(997);
  const numerator = amountInWithFee * reserveOut;
  const denominator = reserveIn * BigInt(1000) + amountInWithFee;
  return numerator / denominator;
}
