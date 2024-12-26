import { getContract, waitTillCompleted } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";
import CurrencyJson from "../../../artifacts/contracts/Currency.sol/Currency.json";
import PairJson from "../../../artifacts/contracts/UniswapV2Pair.sol/UniswapV2Pair.json";
import { createClient } from "../../util/client";

task("burn", "Burn liquidity tokens and print balances and reserves")
  .addParam("pair", "The address of the pair contract")
  .setAction(async (taskArgs, _) => {
    const { wallet, publicClient } = await createClient();

    // Destructure parameters for clarity
    const pairAddress = taskArgs.pair as Address;

    // Attach to the Uniswap V2 Pair contract
    const pair = getContract({
      abi: PairJson.abi,
      address: pairAddress,
      client: wallet.client,
      wallet: wallet,
    });

    const token0 = (await pair.read.token0Id([])) as Address;
    console.log("Token0:", token0);
    const token1 = (await pair.read.token1Id([])) as Address;
    console.log("Token1:", token1);

    // Attach to the Currency contracts
    const token0Contract = getContract({
      abi: CurrencyJson.abi,
      address: token0,
      client: wallet.client,
      wallet: wallet,
    });
    const token1Contract = getContract({
      abi: CurrencyJson.abi,
      address: token1,
      client: wallet.client,
      wallet: wallet,
    });

    const total = await pair.read.getCurrencyTotalSupply([]);
    console.log("Total supply:", total);

    // Fetch and log pair balances before burn
    const pairBalanceToken0 = await token0Contract.read.getCurrencyBalanceOf([
      pairAddress,
    ]);
    const pairBalanceToken1 = await token1Contract.read.getCurrencyBalanceOf([
      pairAddress,
    ]);
    console.log("Pair Balance token0 before burn:", pairBalanceToken0);
    console.log("Pair Balance token1 before burn:", pairBalanceToken1);

    // Fetch and log user balances before burn
    let userBalanceToken0 = await token0Contract.read.getCurrencyBalanceOf([
      wallet.address,
    ]);
    let userBalanceToken1 = await token1Contract.read.getCurrencyBalanceOf([
      wallet.address,
    ]);
    console.log("User Balance token0 before burn:", userBalanceToken0);
    console.log("User Balance token1 before burn:", userBalanceToken1);

    const userLpBalance = (await pair.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as number;
    console.log("Total LP balance for user wallet:", userLpBalance);

    // Execute burn
    console.log("Executing burn...");

    const hash = await pair.write.burn([wallet.address], {
      tokens: [
        {
          id: pairAddress,
          amount: BigInt(userLpBalance),
        },
      ],
    });

    await waitTillCompleted(publicClient, hash);

    console.log("Burn executed.");

    // Log balances after burn
    const balanceToken0 = (await token0Contract.read.getCurrencyBalanceOf([
      pairAddress,
    ])) as number;
    const balanceToken1 = (await token1Contract.read.getCurrencyBalanceOf([
      pairAddress,
    ])) as number;
    console.log("Pair Balance token0 after burn:", balanceToken0);
    console.log("Pair Balance token1 after burn:", balanceToken1);

    userBalanceToken0 = await token0Contract.read.getCurrencyBalanceOf([
      wallet.address,
    ]);
    userBalanceToken1 = await token1Contract.read.getCurrencyBalanceOf([
      wallet.address,
    ]);
    console.log("User Balance token0 after burn:", userBalanceToken0);
    console.log("User Balance token1 after burn:", userBalanceToken1);
  });
