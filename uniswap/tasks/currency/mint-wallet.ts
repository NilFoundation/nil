import { getContract } from "@nilfoundation/niljs";
import type { Address } from "abitype";
import { task } from "hardhat/config";
import CurrencyJson from "../../artifacts/contracts/Currency.sol/Currency.json";
import { createWallet } from "../basic/basic";
import { mintAndSendCurrency } from "../util/currency-utils";

task(
  "mint-wallet",
  "Mint currency from two contracts and send it to a specified wallet",
)
  .addParam("currency", "The contract address of the first currency")
  .addParam("amount", "The amount of currency to mint and send")
  .setAction(async (taskArgs, _) => {
    const walletAddress = process.env.WALLET_ADDR as Address | undefined;

    if (!walletAddress) {
      throw new Error("WALLET_ADDR is not set in environment variables");
    }

    const wallet = await createWallet();

    // Destructure parameters for clarity
    const mintAmount = BigInt(taskArgs.amount);
    const currencyAddress = taskArgs.currency;

    console.log(
      `Starting mint and transfer process for currencies ${currencyAddress}`,
    );
    // Mint and send currency for both contracts using the refactored utility function
    await mintAndSendCurrency({
      wallet: wallet,
      contractAddress: currencyAddress,
      walletAddress,
      mintAmount,
    });

    const contract = getContract({
      abi: CurrencyJson.abi,
      address: currencyAddress,
      client: wallet.client,
      wallet: wallet,
      externalInterface: {
        signer: wallet.signer,
        methods: [],
      },
    });

    // Verify recipient balances
    const balance = await contract.read.getCurrencyBalanceOf([wallet.address]);

    console.log(`Recipient balance after transfer: ${balance}`);
  });
