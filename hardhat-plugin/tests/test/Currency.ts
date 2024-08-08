import { expect } from "chai";
import "@nomicfoundation/hardhat-ethers";
import { deployNilContract } from "../src/deployUtil";
import { PublicClient, HttpTransport } from "@nilfoundation/niljs";

describe("Currency contract", () => {
    let currencyAddress: string;
    let client: any;

    before(async () => {
        if (!process.env.NIL_RPC_ENDPOINT) {
            throw new Error('NIL_RPC_ENDPOINT is not defined');
        }

        client = new PublicClient({
            transport: new HttpTransport({
                endpoint: process.env.NIL_RPC_ENDPOINT,
            }),
            shardId: 1,
        });
    });

    it("Should create a currency and fetch balance", async () => {
        // Deploy Currency contract
        const { deployedContract: currency, contractAddress: currencyAddr } = await deployNilContract("Currency", ["TestCurrency"]);
        currencyAddress = currencyAddr;
        console.log("Currency deployed at:", currencyAddress);

        // Call the create method on the Currency contract
        const amount = 1000;
        await currency.create(amount);

        // Fetch and print the currency balance using nil.js
        const tokens = await client.getCurrencies(currencyAddress, "latest");
        console.log("Tokens:", tokens);
        let found = false;
        let tokenId;
        for (const [id, balance] of Object.entries(tokens)) {
            console.log(`Token ID: ${id}, Balance: ${balance}`);
            if (balance === 1000n) {
                tokenId= id;
                found = true;
                break;
            }
        }
        expect(found).to.be.true;
        found = false;

        // Deploy destination contract (e.g., IncrementerPayable)
        const { deployedContract: destination, contractAddress: destinationAddr } = await deployNilContract("IncrementerPayable", []);
        console.log("Destination contract deployed at:", destinationAddr);

        // Call the transfer method on the Currency contract
        const transferAmount = 500;
        await currency.transfer(destinationAddr, tokenId, transferAmount);

        // Fetch and print the currency balance again to verify transfer
        const updatedTokens = await client.getCurrencies(currencyAddress, "latest");
        console.log("Updated Tokens:", updatedTokens);

        // Check if the destination contract received the tokens (assuming it has a method to fetch balance)
        const destinationTokens = await client.getCurrencies(destinationAddr, "latest");
        console.log("Destination Tokens:", destinationTokens);

        for (const [id, balance] of Object.entries(destinationTokens)) {
            console.log(`Token ID: ${id}, Balance: ${balance}`);
            if (balance === 500n) {
                found = true;
                break;
            }
        }
        expect(found).to.be.true;
    });
});
