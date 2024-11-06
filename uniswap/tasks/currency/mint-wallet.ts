import { task } from "hardhat/config";
import type { Currency } from "../../typechain-types";
import { createClient } from "../util/client";
import {
  faucetWithdrawal,
  mintAndSendCurrency,
  sleep,
} from "../util/currencyUtils";

task(
  "mint-wallet",
  "Mint currency from two contracts and send it to a specified wallet",
)
  .addParam("currency0", "The contract address of the first currency")
  .addParam("currency1", "The contract address of the second currency")
  .addParam("amount", "The amount of currency to mint and send")
  .setAction(async (taskArgs, hre) => {
    const walletAddress = process.env.WALLET_ADDR;

    if (!walletAddress) {
      throw new Error("WALLET_ADDR is not set in environment variables");
    }

    const faucetAddress = process.env.FAUCET_ADDR;

    if (!faucetAddress) {
      throw new Error("FAUCET_ADDR is not set in environment variables");
    }

    const { wallet, publicClient, signer } = await createClient();

    // Destructure parameters for clarity
    const mintAmount = BigInt(taskArgs.amount);
    const currency0Address = taskArgs.currency0;
    const currency1Address = taskArgs.currency1;

    console.log(
      `Starting mint and transfer process for currencies ${currency0Address} and ${currency1Address}`,
    );

    // Withdraw from faucet for both currencies
    console.log(
      `Withdrawing from faucet for currency 0 (${currency0Address})...`,
    );
    await faucetWithdrawal(
      currency0Address,
      100000000000n,
      faucetAddress,
      hre,
      publicClient,
    );

    // Sleep for 2 second
    console.log("Waiting 2 second to prevent sequence issues..");
    await sleep(2000);

    console.log(
      `Withdrawing from faucet for currency 1 (${currency1Address})...`,
    );
    await faucetWithdrawal(
      currency1Address,
      100000000000n,
      faucetAddress,
      hre,
      publicClient,
    );

    // Attach to Currency contracts
    const CurrencyFactory = await hre.ethers.getContractFactory("Currency");
    const currency0 = CurrencyFactory.attach(currency0Address) as Currency;
    const currency1 = CurrencyFactory.attach(currency1Address) as Currency;

    // Mint and send currency for both contracts using the refactored utility function
    console.log(`Minting and sending currency 0 (${currency0Address})...`);
    await mintAndSendCurrency({
      publicClient,
      signer,
      currencyContract: currency0,
      contractAddress: currency0Address,
      walletAddress,
      mintAmount,
      hre,
    });

    // Sleep for 2 second
    console.log("Waiting 2 second to prevent sequence issues...");
    await sleep(2000);

    console.log(`Minting and sending currency 1 (${currency1Address})...`);
    await mintAndSendCurrency({
      publicClient,
      signer,
      currencyContract: currency1,
      contractAddress: currency1Address,
      walletAddress,
      mintAmount,
      hre,
    });

    // Verify recipient balances
    const recipientBalanceCurrency0 =
      await currency0.getCurrencyBalanceOf(walletAddress);
    const recipientBalanceCurrency1 =
      await currency1.getCurrencyBalanceOf(walletAddress);

    console.log(
      `Recipient balance after transfer - Currency0: ${recipientBalanceCurrency0}, Currency1: ${recipientBalanceCurrency1}`,
    );
  });
