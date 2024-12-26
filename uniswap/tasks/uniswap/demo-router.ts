import {
  Faucet,
  bytesToHex,
  getContract,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import type { Abi } from "abitype";
import { task } from "hardhat/config";
import CurrencyJson from "../../artifacts/contracts/Currency.sol/Currency.json";
import FactoryJson from "../../artifacts/contracts/UniswapV2Factory.sol/UniswapV2Factory.json";
import PairJson from "../../artifacts/contracts/UniswapV2Pair.sol/UniswapV2Pair.json";
import RouterJson from "../../artifacts/contracts/UniswapV2Router01.sol/UniswapV2Router01.json";
import { createWallet } from "../basic/basic";
import { mintAndSendCurrency, sleep } from "../util/currency-utils";
import { deployNilContract } from "../util/deploy";
import { calculateOutputAmount } from "../util/math";

task("demo-router", "Run demo with Uniswap Router").setAction(
  async (taskArgs, _) => {
    const shardId = 1;
    const mintAmount = BigInt(100000);
    const mintCurrency0Amount = 10000;
    const mintCurrency1Amount = 10000;
    const swapAmount = 1000;

    const wallet = await createWallet();
    const faucet = new Faucet(wallet.client);

    const { contract: factory, address: factoryAddress } =
      await deployNilContract(
        wallet,
        FactoryJson.abi as Abi,
        FactoryJson.bytecode,
        [wallet.address],
        wallet.shardId,
        [],
      );

    const { contract: currency0, address: currency0Address } =
      await deployNilContract(
        wallet,
        CurrencyJson.abi as Abi,
        CurrencyJson.bytecode,
        ["Currency", bytesToHex(wallet.signer.getPublicKey())],
        wallet.shardId,
        ["mintCurrency", "sendCurrency"],
      );
    console.log("Currency contract deployed at address: " + currency0Address);

    await faucet.withdrawToWithRetry(currency0Address);

    const { contract: currency1, address: currency1Address } =
      await deployNilContract(
        wallet,
        CurrencyJson.abi as Abi,
        CurrencyJson.bytecode,
        ["Currency", bytesToHex(wallet.signer.getPublicKey())],
        wallet.shardId,
        ["mintCurrency", "sendCurrency"],
      );
    console.log("Currency contract deployed at address: " + currency1Address);

    await faucet.withdrawToWithRetry(currency1Address);

    console.log("Factory deployed " + factoryAddress);
    console.log("Currency0 deployed " + currency0Address);
    console.log("Currency1 deployed " + currency1Address);

    const { contract: router, address: routerAddress } =
      await deployNilContract(
        wallet,
        RouterJson.abi as Abi,
        RouterJson.bytecode,
        [],
        wallet.shardId,
        [],
      );

    console.log("Router deployed " + routerAddress);

    // 1. CREATE PAIR
    const pairTxHash = await factory.write.createPair([
      currency0Address,
      currency1Address,
      Math.floor(Math.random() * 10000000),
      shardId,
    ]);

    await waitTillCompleted(wallet.client, pairTxHash);
    await sleep(3000);

    const pairAddress = await factory.read.getTokenPair([
      currency0Address,
      currency1Address,
    ]);

    // Log the pair address
    console.log(`Pair created successfully at address: ${pairAddress}`);

    const pair = getContract({
      abi: PairJson.abi,
      address: pairAddress,
      client: wallet.client,
      wallet: wallet,
    });

    // Initialize the pair with currency addresses and IDs
    await pair.write.initialize([currency0Address, currency1Address]);

    console.log(`Pair initialized successfully at address: ${pairAddress}`);

    // 2. MINT CURRENCIES
    console.log(
      `Minting ${mintAmount} Currency0 to wallet ${wallet.address}...`,
    );
    await mintAndSendCurrency({
      wallet,
      contractAddress: currency0Address,
      walletAddress: wallet.address,
      mintAmount,
    });

    // Mint and send Currency1
    console.log(
      `Minting ${mintAmount} Currency1 to wallet ${wallet.address}...`,
    );
    await mintAndSendCurrency({
      wallet,
      contractAddress: currency1Address,
      walletAddress: wallet.address,
      mintAmount,
    });

    // Verify the balance of the recipient wallet for both currencies
    const recipientBalanceCurrency0 = await currency0.read.getCurrencyBalanceOf(
      [wallet.address],
    );
    const recipientBalanceCurrency1 = await currency1.read.getCurrencyBalanceOf(
      [wallet.address],
    );

    console.log(
      `Recipient balance after transfer - Currency0: ${recipientBalanceCurrency0}, Currency1: ${recipientBalanceCurrency1}`,
    );

    // 3. ROUTER: ADD LIQUIDITY

    // Mint liquidity
    console.log("Adding liquidity...");

    const hash = await router.write.addLiquidity(
      [pairAddress, wallet.address],
      {
        tokens: [
          {
            id: currency0Address,
            amount: BigInt(mintCurrency0Amount),
          },
          {
            id: currency1Address,
            amount: BigInt(mintCurrency1Amount),
          },
        ],
      },
    );

    await waitTillCompleted(wallet.client, hash);

    // Log balances in the pair contract
    const pairCurrency0Balance = (await currency0.read.getCurrencyBalanceOf([
      pairAddress,
    ])) as number;
    console.log("Pair Balance of Currency0:", pairCurrency0Balance);

    const pairCurrency1Balance = (await currency1.read.getCurrencyBalanceOf([
      pairAddress,
    ])) as number;
    console.log("Pair Balance of Currency1:", pairCurrency1Balance);

    console.log("Liquidity added...");

    // Retrieve and log reserves from the pair
    // @ts-ignore
    const [reserve0, reserve1] = await pair.read.getReserves([]);
    console.log(
      `ADDLIQUIDITY RESULT: Reserves - Currency0: ${reserve0.toString()}, Currency1: ${reserve1.toString()}`,
    );

    // Check and log liquidity provider balance
    const lpBalance = (await pair.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    console.log(
      "ADDLIQUIDITY RESULT: Liquidity provider balance in wallet:",
      lpBalance.toString(),
    );

    // Retrieve and log total supply for the pair
    const totalSupply = (await pair.read.getCurrencyTotalSupply([])) as bigint;
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

    // Send currency0 to the pair contract
    const hash2 = await router.write.swap(
      [wallet.address, pairAddress, 0, expectedOutputAmount],
      {
        tokens: [
          {
            id: currency0Address,
            amount: BigInt(swapAmount),
          },
        ],
      },
    );

    await waitTillCompleted(wallet.client, hash2);

    console.log(
      `Sent ${swapAmount.toString()} of currency0 to the pair contract. Tx - ${hash2}`,
    );

    console.log("Swap executed successfully.");

    // Log balances after the swap
    const balanceCurrency0After = (await currency0.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    const balanceCurrency1After = (await currency1.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    console.log(
      "SWAP RESULT: Balance of currency0 after swap:",
      balanceCurrency0After.toString(),
    );
    console.log(
      "SWAP RESULT: Balance of currency1 after swap:",
      balanceCurrency1After.toString(),
    );

    // 5. ROUTER: REMOVE LIQUIDITY
    const total = (await pair.read.getCurrencyTotalSupply([])) as bigint;
    console.log("Total supply:", total.toString());

    // Fetch and log pair balances before burn
    const pairBalanceToken0 = (await currency0.read.getCurrencyBalanceOf([
      pairAddress,
    ])) as bigint;
    const pairBalanceToken1 = (await currency1.read.getCurrencyBalanceOf([
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
    let userBalanceToken0 = (await currency0.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    let userBalanceToken1 = (await currency1.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    console.log(
      "User Balance token0 before burn:",
      userBalanceToken0.toString(),
    );
    console.log(
      "User Balance token1 before burn:",
      userBalanceToken1.toString(),
    );

    const userLpBalance = (await pair.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as bigint;
    console.log("Total LP balance for user wallet:", userLpBalance.toString());
    // Execute burn
    console.log("Executing burn...");
    // Send LP tokens to the user wallet
    const hash3 = await router.write.removeLiquidity(
      [pairAddress, wallet.address],
      {
        tokens: [
          {
            id: pairAddress,
            amount: BigInt(userLpBalance),
          },
        ],
      },
    );

    await waitTillCompleted(wallet.client, hash3);

    console.log("Burn executed.");

    // Log balances after burn
    const balanceToken0 = (await currency0.read.getCurrencyBalanceOf([
      pairAddress,
    ])) as bigint;
    const balanceToken1 = (await currency1.read.getCurrencyBalanceOf([
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

    userBalanceToken0 = await currency0.read.getCurrencyBalanceOf([
      wallet.address,
    ]);
    userBalanceToken1 = await currency1.read.getCurrencyBalanceOf([
      wallet.address,
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
