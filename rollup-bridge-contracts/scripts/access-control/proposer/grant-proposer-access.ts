import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../../deploy/config/config-helper";
import { getAllProposers } from "./get-all-proposers";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

// npx hardhat run scripts/access-control/proposer/grant-proposer-access.ts --network sepolia

// Function to grant proposer access
export async function grantProposerAccess(proposerAddress: string) {

    const networkName = network.name;
    const config = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error("Invalid nilRollupProxy address in config");
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    console.log(`nilRollupProxy on network: ${networkName} at address: ${config.nilRollupProxy} is granting proposer-access to ${proposerAddress}`);

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(config.nilRollupProxy, abi, signer) as Contract;

    // Grant proposer access
    const tx = await nilRollupInstance.grantProposerAccess(proposerAddress);
    await tx.wait();

    console.log(`Proposer access granted to ${proposerAddress}`);

    const proposers = await getAllProposers();
    console.log(`proposers are: ${JSON.stringify(proposers)}`);
}

// Main function to call the grantProposerAccess function
async function main() {
    const proposerAddress = "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206";
    await grantProposerAccess(proposerAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});