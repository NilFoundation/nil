import { waitTillCompleted } from "@nilfoundation/niljs";
import { Faucet, getContract } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";
import CurrencyJson from "../../../artifacts/contracts/Currency.sol/Currency.json";
import PairJson from "../../../artifacts/contracts/UniswapV2Pair.sol/UniswapV2Pair.json";
import { createWallet } from "../../basic/basic";
import { sleep } from "../../util/currency-utils";

task("mint", "Mint currencies and add liquidity to the pair")
  .addParam("pair", "The address of the pair contract")
  .addParam("amount0", "The amount of the first currency to mint")
  .addParam("amount1", "The amount of the second currency to mint")
  .setAction(async (taskArgs, _) => {
    const wallet = await createWallet();

    // Destructure parameters for clarity
    const pairAddress = taskArgs.pair as Address;
    const amount0 = taskArgs.amount0 as number;
    const amount1 = taskArgs.amount1 as number;

    const pair = getContract({
      abi: PairJson.abi,
      address: pairAddress,
      client: wallet.client,
      wallet: wallet,
    });

    // Fetch currency addresses from the pair contract
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

    // Mint liquidity
    const faucet = new Faucet(wallet.client);
    await faucet.withdrawToWithRetry(pairAddress);
    await sleep(3000);
    console.log("Minting pair tokens...");

    const hash = await pair.write.mint([wallet.address], {
      tokens: [
        {
          id: currency0Address,
          amount: BigInt(amount0),
        },
        {
          id: currency1Address,
          amount: BigInt(amount1),
        },
      ],
    });

    await waitTillCompleted(wallet.client, hash);

    // Log balances in the pair contract
    const pairCurrency0Balance = await currency0.read.getCurrencyBalanceOf([
      pairAddress,
    ]);
    console.log("Pair Balance 0:", pairCurrency0Balance);

    const pairCurrency1Balance = await currency1.read.getCurrencyBalanceOf([
      pairAddress,
    ]);
    console.log("Pair Balance 1:", pairCurrency1Balance);

    console.log("Liquidity added...");

    // Retrieve and log reserves from the pair
    const [reserve0, reserve1] = (await pair.read.getReserves([])) as number[];
    console.log(
      `Reserves - Currency0: ${reserve0.toString()}, Currency1: ${reserve1.toString()}`,
    );

    // Check and log liquidity provider balance
    const lpBalance = (await pair.read.getCurrencyBalanceOf([
      wallet.address,
    ])) as number;
    console.log("Liquidity provider balance in wallet:", lpBalance.toString());

    // Retrieve and log total supply for the pair
    const totalSupply = (await pair.read.getCurrencyTotalSupply([])) as number;
    console.log("Total supply of pair tokens:", totalSupply.toString());
  });
