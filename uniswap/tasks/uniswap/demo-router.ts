import {
  bytesToHex,
  getContract,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import type { Abi } from "abitype";
import { task } from "hardhat/config";
import { createSmartAccount, topUpSmartAccount } from "../basic/basic";
import { deployNilContract } from "../util/deploy";
import { calculateOutputAmount } from "../util/math";
import { mintAndSendToken } from "../util/token-utils";

task("demo-router", "Run demo with Uniswap Router").setAction(
  async (taskArgs, hre) => {
    const shardId = 1;
    const mintAmount = BigInt(100000);
    const mintToken0Amount = 10000;
    const mintToken1Amount = 10000;
    const swapAmount = 1000;

    const smartAccount = await hre.nil.getSmartAccount();
    const client = await hre.nil.getPublicClient();

    const TokenJson = require("../../artifacts/contracts/Token.sol/Token.json");
    const FactoryJson = require("../../artifacts/contracts/UniswapV2Factory.sol/UniswapV2Factory.json");
    const PairJson = require("../../artifacts/contracts/UniswapV2Pair.sol/UniswapV2Pair.json");
    const RouterJson = require("../../artifacts/contracts/UniswapV2Router01.sol/UniswapV2Router01.json");

    const { contract: factory, address: factoryAddress } =
      await deployNilContract(
        smartAccount,
        FactoryJson.abi as Abi,
        FactoryJson.bytecode,
        [smartAccount.address],
        smartAccount.shardId,
        [],
      );

    const { contract: token0, address: token0Address } =
      await deployNilContract(
        smartAccount,
        TokenJson.abi as Abi,
        TokenJson.bytecode,
        ["Token", bytesToHex(smartAccount.signer.getPublicKey())],
        smartAccount.shardId,
        ["mintToken", "sendToken"],
      );
    console.log("Token contract deployed at address: " + token0Address);

    await topUpSmartAccount(token0Address);

    const { contract: token1, address: token1Address } =
      await deployNilContract(
        smartAccount,
        TokenJson.abi as Abi,
        TokenJson.bytecode,
        ["Token", bytesToHex(smartAccount.signer.getPublicKey())],
        smartAccount.shardId,
        ["mintToken", "sendToken"],
      );
    console.log("Token contract deployed at address: " + token1Address);

    await topUpSmartAccount(token1Address);

    console.log("Factory deployed " + factoryAddress);
    console.log("Token0 deployed " + token0Address);
    console.log("Token1 deployed " + token1Address);

    const { contract: router, address: routerAddress } =
      await deployNilContract(
        smartAccount,
        RouterJson.abi as Abi,
        RouterJson.bytecode,
        [],
        smartAccount.shardId,
        [],
      );

    console.log("Router deployed " + routerAddress);

    // 1. CREATE PAIR
    const pairTxHash = await factory.write.createPair([
      token0Address,
      token1Address,
      Math.floor(Math.random() * 10000000),
      shardId,
    ]);

    await waitTillCompleted(smartAccount.client, pairTxHash);

    const pairAddress = await factory.read.getTokenPair([
      token0Address,
      token1Address,
    ]);

    // Log the pair address
    console.log(`Pair created successfully at address: ${pairAddress}`);

    const pair = getContract({
      abi: PairJson.abi,
      address: pairAddress,
      client: smartAccount.client,
      smartAccount: smartAccount,
    });

    // Initialize the pair with token addresses and IDs
    await pair.write.initialize([token0Address, token1Address]);

    console.log(`Pair initialized successfully at address: ${pairAddress}`);

    // 2. MINT TOKENS
    console.log(
      `Minting ${mintAmount} Token0 to smart account ${smartAccount.address}...`,
    );
    await mintAndSendToken({
      smartAccount,
      contractAddress: token0Address,
      smartAccountAddress: smartAccount.address,
      mintAmount,
    });

    // Mint and send Token1
    console.log(
      `Minting ${mintAmount} Token1 to smart account ${smartAccount.address}...`,
    );
    await mintAndSendToken({
      smartAccount,
      contractAddress: token1Address,
      smartAccountAddress: smartAccount.address,
      mintAmount,
    });

    // Verify the balance of the recipient smart account for both tokens
    const recipientBalanceToken0 = await token0.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    const recipientBalanceToken1 = await token1.read.getTokenBalanceOf([
      smartAccount.address,
    ]);

    console.log(
      `Recipient balance after transfer - Token0: ${recipientBalanceToken0}, Token1: ${recipientBalanceToken1}`,
    );

    // 3. ROUTER: ADD LIQUIDITY

    // Mint liquidity
    console.log("Adding liquidity...");

    const hash = await router.write.addLiquidity(
      [pairAddress, smartAccount.address],
      {
        tokens: [
          {
            id: token0Address,
            amount: BigInt(mintToken0Amount),
          },
          {
            id: token1Address,
            amount: BigInt(mintToken1Amount),
          },
        ],
      },
    );

    await waitTillCompleted(smartAccount.client, hash);

    // Log balances in the pair contract
    const pairToken0Balance = (await token0.read.getTokenBalanceOf([
      pairAddress,
    ])) as number;
    console.log("Pair Balance of Token0:", pairToken0Balance);

    const pairToken1Balance = (await token1.read.getTokenBalanceOf([
      pairAddress,
    ])) as number;
    console.log("Pair Balance of Token1:", pairToken1Balance);

    console.log("Liquidity added...");

    // Retrieve and log reserves from the pair
    // @ts-ignore
    const [reserve0, reserve1] = await pair.read.getReserves([]);
    console.log(
      `ADDLIQUIDITY RESULT: Reserves - Token0: ${reserve0.toString()}, Token1: ${reserve1.toString()}`,
    );

    // Check and log liquidity provider balance
    const lpBalance = (await pair.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    console.log(
      "ADDLIQUIDITY RESULT: Liquidity provider balance in smart account:",
      lpBalance.toString(),
    );

    // Retrieve and log total supply for the pair
    const totalSupply = (await pair.read.getTokenTotalSupply([])) as bigint;
    console.log(
      "ADDLIQUIDITY RESULT: Total supply of pair tokens:",
      totalSupply.toString(),
    );

    // 4. ROUTER: SWAP
    const expectedOutputAmount = calculateOutputAmount(
      BigInt(swapAmount),
      reserve0,
      reserve1,
    );
    console.log(
      "Expected output amount for swap:",
      expectedOutputAmount.toString(),
    );

    // Log balances before the swap
    const balanceToken0Before = (await token0.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    const balanceToken1Before = (await token1.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    console.log(
      "Balance of token0 before swap:",
      balanceToken0Before.toString(),
    );
    console.log(
      "Balance of token1 before swap:",
      balanceToken1Before.toString(),
    );

    // Execute the swap
    console.log("Executing swap...");

    // Send token0 to the pair contract
    const hash2 = await router.write.swap(
      [smartAccount.address, pairAddress, 0, expectedOutputAmount],
      {
        tokens: [
          {
            id: token0Address,
            amount: BigInt(swapAmount),
          },
        ],
      },
    );

    await waitTillCompleted(smartAccount.client, hash2);

    console.log(
      `Sent ${swapAmount.toString()} of token0 to the pair contract. Tx - ${hash2}`,
    );

    console.log("Swap executed successfully.");

    // Log balances after the swap
    const balanceToken0After = (await token0.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    const balanceToken1After = (await token1.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    console.log(
      "SWAP RESULT: Balance of token0 after swap:",
      balanceToken0After.toString(),
    );
    console.log(
      "SWAP RESULT: Balance of token1 after swap:",
      balanceToken1After.toString(),
    );

    // 5. ROUTER: REMOVE LIQUIDITY
    const total = (await pair.read.getTokenTotalSupply([])) as bigint;
    console.log("Total supply:", total.toString());

    // Fetch and log pair balances before burn
    const pairBalanceToken0 = (await token0.read.getTokenBalanceOf([
      pairAddress,
    ])) as bigint;
    const pairBalanceToken1 = (await token1.read.getTokenBalanceOf([
      pairAddress,
    ])) as bigint;
    console.log(
      "Pair Balance token0 before burn:",
      pairBalanceToken0.toString(),
    );
    console.log(
      "Pair Balance token1 before burn:",
      pairBalanceToken1.toString(),
    );

    // Fetch and log user balances before burn
    let userBalanceToken0 = (await token0.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    let userBalanceToken1 = (await token1.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    console.log(
      "User Balance token0 before burn:",
      userBalanceToken0.toString(),
    );
    console.log(
      "User Balance token1 before burn:",
      userBalanceToken1.toString(),
    );

    const userLpBalance = (await pair.read.getTokenBalanceOf([
      smartAccount.address,
    ])) as bigint;
    console.log(
      "Total LP balance for user smart account:",
      userLpBalance.toString(),
    );
    // Execute burn
    console.log("Executing burn...");
    // Send LP tokens to the user smart account
    const hash3 = await router.write.removeLiquidity(
      [pairAddress, smartAccount.address],
      {
        tokens: [
          {
            id: pairAddress,
            amount: BigInt(userLpBalance),
          },
        ],
      },
    );

    await waitTillCompleted(smartAccount.client, hash3);

    console.log("Burn executed.");

    // Log balances after burn
    const balanceToken0 = (await token0.read.getTokenBalanceOf([
      pairAddress,
    ])) as bigint;
    const balanceToken1 = (await token1.read.getTokenBalanceOf([
      pairAddress,
    ])) as bigint;
    console.log(
      "REMOVELIQUIDITY RESULT: Pair Balance token0 after burn:",
      balanceToken0.toString(),
    );
    console.log(
      "REMOVELIQUIDITY RESULT: Pair Balance token1 after burn:",
      balanceToken1.toString(),
    );

    userBalanceToken0 = await token0.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    userBalanceToken1 = await token1.read.getTokenBalanceOf([
      smartAccount.address,
    ]);
    console.log(
      "REMOVELIQUIDITY RESULT: User Balance token0 after burn:",
      userBalanceToken0.toString(),
    );
    console.log(
      "REMOVELIQUIDITY RESULT: User Balance token1 after burn:",
      userBalanceToken1.toString(),
    );

    // Fetch and log reserves after burn
    const reserves = await pair.read.getReserves([]);
    console.log(
      "REMOVELIQUIDITY RESULT: Reserves from pair after burn:",
      // @ts-ignore
      reserves[0].toString(),
      // @ts-ignore
      reserves[1].toString(),
    );
  },
);
