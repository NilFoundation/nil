import { task } from "hardhat/config";
import type { Currency } from "../../typechain-types";

task("currency_info", "Retrieve currency name and ID")
  .addParam("address", "The address of the deployed currency contract")
  .setAction(async (taskArgs, hre) => {
    const address = taskArgs.address;

    // Attach the Currency contract at the provided address
    const Currency = await hre.ethers.getContractFactory("Currency");
    const currency = Currency.attach(address) as Currency;

    // Retrieve the currency's name
    const currencyName = await currency.getCurrencyName();
    console.log("Currency Name: " + currencyName);

    // Retrieve the currency's unique ID
    const currencyId = await currency.getCurrencyId();
    console.log("Currency ID: " + currencyId);

    // Retrieve the contract's own currency balance
    const balance = await currency.getOwnCurrencyBalance();
    console.log("Currency Balance: " + balance);
  });
