import type { Address } from "abitype";
import { task } from "hardhat/config";
import { createClient } from "../../util/client";
import { waitTillCompleted } from "@nilfoundation/niljs";

task("burn", "Burn liquidity tokens and print balances and reserves")
  .addParam("pair", "The address of the pair contract")
  .setAction(async (taskArgs, hre) => {
    const { smartAccount, publicClient } = await createClient();

    const pairAddress = taskArgs.pair as Address;
    const pair = await hre.nil.getContractAt("UniswapV2Pair", pairAddress, {});

    const token0 = (await pair.read.token0Id([])) as Address;
    console.log("Token0:", token0);
    const token1 = (await pair.read.token1Id([])) as Address;
    console.log("Token1:", token1);

    const TokenJson = require("../../artifacts/contracts/Token.sol/Token.json");
    // Attach to the Token contracts
    const token0Contract = await hre.nil.getContractAt("Token", token0, {});
    const token1Contract = await hre.nil.getContractAt("Token", token1, {});

    const total = await pair.read.getTokenTotalSupply([]);
    console.log("Total supply:", total);

    const pairBalanceToken0 = await token0Contract.read.getTokenBalanceOf([
      pairAddress,
    ]);
    const pairBalanceToken1 = await token1Contract.read.getTokenBalanceOf([
      pairAddress,
    ]);
    console.log("Pair Balance token0 before burn:", pairBalanceToken0);
    console.log("Pair Balance token1 before burn:", pairBalanceToken1);

    // Fetch and log user balances before burn
    let userBalanceToken0 = await token0Contract.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    let userBalanceToken1 = await token1Contract.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    console.log("User Balance token0 before burn:", userBalanceToken0);
    console.log("User Balance token1 before burn:", userBalanceToken1);

    const userLpBalance = (await pair.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as number;
    console.log("Total LP balance for user smart account:", userLpBalance);

    // Execute burn
    console.log("Executing burn...");

    const hash = await pair.write.burn([smartAccount.address], {
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
    const balanceToken0 = (await token0Contract.read.getTokenBalanceOf([
      pairAddress,
    ])) as number;
    const balanceToken1 = (await token1Contract.read.getTokenBalanceOf([
      pairAddress,
    ])) as number;
    console.log("Pair Balance token0 after burn:", balanceToken0);
    console.log("Pair Balance token1 after burn:", balanceToken1);

    userBalanceToken0 = await token0Contract.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    userBalanceToken1 = await token1Contract.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    console.log("User Balance token0 after burn:", userBalanceToken0);
    console.log("User Balance token1 after burn:", userBalanceToken1);
  });
