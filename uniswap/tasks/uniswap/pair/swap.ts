import { waitTillCompleted } from "@nilfoundation/niljs";
import { getContract } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";
// import CurrencyJson from "../../../artifacts/contracts/Currency.sol/Currency.json";
// import PairJson from "../../../artifacts/contracts/UniswapV2Pair.sol/UniswapV2Pair.json";
import { createWallet } from "../../basic/basic";

task("swap", "Swap currency0 for currency1 in the Uniswap pair")
  .addParam("pair", "The address of the Uniswap pair contract")
  .addParam("amount", "The amount of currency0 to swap")
  .setAction(async (taskArgs, _) => {
    const wallet = await createWallet();

    // Destructure parameters for clarity
    const pairAddress = taskArgs.pair as Address;
    const swapAmount = BigInt(taskArgs.amount);

    const pair = getContract({
      abi: PairJson.abi,
      address: pairAddress,
      client: wallet.client,
      wallet: wallet,
    });

    // Retrieve currency addresses from the pair contract
    const currency0Address = (await pair.read.token0Id([])) as Address;
    const currency1Address = (await pair.read.token1Id([])) as Address;

    console.log("Currency 0 Address:", currency0Address);
    console.log("Currency 1 Address:", currency1Address);

    // Attach to the Currency contracts
    const currency0 = getContract({
      abi: CurrencyJson.abi,
      address: currency0Address,
      client: wallet.client,
      wallet: wallet,
    });
    const currency1 = getContract({
      abi: CurrencyJson.abi,
      address: currency1Address,
      client: wallet.client,
      wallet: wallet,
    });

    // Retrieve reserves from the pair
    const reserves = await pair.read.getReserves([]);
    // @ts-ignore
    const reserve0 = reserves[0] as bigint;
    // @ts-ignore
    const reserve1 = reserves[1] as bigint;
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
    const balanceCurrency0Before = (await currency0.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    const balanceCurrency1Before = (await currency1.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
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

    const hash = await pair.write.swap(
      [0, expectedOutputAmount, wallet.address],
      {
        tokens: [
          {
            id: currency0Address,
            amount: swapAmount,
          },
        ],
      },
    );

    await waitTillCompleted(wallet.client, hash);

    console.log("Swap executed successfully.");

    // Log balances after the swap
    const balanceCurrency0After = (await currency0.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    const balanceCurrency1After = (await currency1.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
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
