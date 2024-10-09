import { expect } from "chai";
import "@nomicfoundation/hardhat-ethers";
import { deployNilContract } from "../src/deployUtil";
import { ethers } from "hardhat";

describe("Currency contract", () => {
    it("Should deploy, transfer currency, and verify balances", async () => {
        // Initialize values for testing
        const initialSupply = 10000;

        // Deploy the Currency contract with initial supply and name
        const {
            deployedContract: currencyBase,
            contractAddress: currencyBaseAddr
        } = await deployNilContract("Currency", [initialSupply]);
        console.log("Currency deployed at:", currencyBaseAddr);

        const balance = await currencyBase.getOwnCurrencyBalance();
        console.log("Currency balance:", balance.toString());
        expect(balance).to.equal(initialSupply);

        // Fetch and verify the currency ID
        const currencyId = await currencyBase.getCurrencyId();
        console.log("Currency ID:", currencyId);

        // Compare the expected and actual currency IDs
        expect(currencyId.toString()).to.equal(currencyBaseAddr.toString());

        // Deploy the IncrementerPayable contract
        const { deployedContract: incrementer, contractAddress: incrementerAddr } = await deployNilContract("IncrementerPayable", []);
        console.log("IncrementerPayable deployed at:", incrementerAddr);

        // Transfer currency from the Currency contract to the IncrementerPayable contract
        const transferAmount = 500;
        await currencyBase.transferCurrency(incrementerAddr, currencyId, transferAmount);

        // After the transfer, verify that the balance in the Currency contract has decreased
        const updatedBalance = await currencyBase.getOwnCurrencyBalance();
        console.log("Updated Currency balance:", updatedBalance.toString());
        expect(updatedBalance).to.equal(initialSupply - transferAmount);

        // Fetch and verify that the total supply remains unchanged
        const totalSupply = await currencyBase.getCurrencyTotalSupply();
        console.log("Total Supply:", totalSupply.toString());
        expect(totalSupply).to.equal(initialSupply);

        // Verify the balance of the IncrementerPayable contract using getCurrencyBalanceOf
        const incrementerBalance = await currencyBase.getCurrencyBalanceOf(incrementerAddr);
        console.log("IncrementerPayable balance:", incrementerBalance.toString());
        expect(incrementerBalance).to.equal(transferAmount);
    });
});
