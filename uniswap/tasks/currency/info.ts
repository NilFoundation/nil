import { getContract } from "@nilfoundation/niljs";
import { task } from "hardhat/config";
import CurrencyJson from "../../artifacts/contracts/Currency.sol/Currency.json";
import { createWallet } from "../basic/basic";

task("currency-info", "Retrieve currency name and ID")
  .addParam("address", "The address of the deployed currency contract")
  .setAction(async (taskArgs, _) => {
    const wallet = await createWallet();

    const contract = getContract({
      abi: CurrencyJson.abi,
      address: taskArgs.address,
      client: wallet.client,
      wallet: wallet,
      externalInterface: {
        signer: wallet.signer,
        methods: [],
      },
    });

    // Retrieve the currency's name
    const currencyName = await contract.read.getCurrencyName([]);
    console.log("Currency Name: " + currencyName);

    // Retrieve the currency's unique ID
    const currencyId = await contract.read.getCurrencyId([]);
    console.log("Currency ID: " + currencyId);

    // Retrieve the contract's own currency balance
    const balance = await contract.read.getOwnCurrencyBalance([]);
    const balance2 = await contract.read.getCurrencyBalanceOf([wallet.address]);
    console.log("Currency Balance: " + balance + " " + balance2);
  });
