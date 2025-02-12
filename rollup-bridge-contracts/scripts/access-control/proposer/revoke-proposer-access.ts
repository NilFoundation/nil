import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../../deploy/config/config-helper";
import { getAllProposers } from "./get-all-proposers";
import { isAProposer } from "./is-a-proposer";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

// npx hardhat run scripts/access-control/proposer/revoke-proposer-access.ts --network sepolia

// Function to revoke proposer access
export async function revokeProposerAccess(proposerAddress: string) {

    const networkName = network.name;
    const config = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error("Invalid nilRollupProxy address in config");
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    console.log(`nilRollupProxy on network: ${networkName} at address: ${config.nilRollupProxy} is revoking proposer-access to ${proposerAddress}`);

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(config.nilRollupProxy, abi, signer) as Contract;

    let isAProposerResponse: Boolean = await isAProposer(proposerAddress);

    if (!isAProposerResponse) {
        throw new Error(`account: ${proposerAddress} is not a proposer. so revokeProposer cannot be initiated`);
    }

    // Grant proposer access
    const tx = await nilRollupInstance.revokeProposerAccess(proposerAddress);
    await tx.wait();

    console.log(`Proposer access revoked to ${proposerAddress}`);

    const proposers = await getAllProposers();
    console.log(`latest list of proposers are: ${JSON.stringify(proposers)}`);

    isAProposerResponse = await isAProposer(proposerAddress);

    if (isAProposerResponse) {
        throw new Error(`revokeProposer failed. account: ${proposerAddress} is still a proposer.`);
    }
}

// Main function to call the revokeProposerAccess function
async function main() {
    const proposerAddress = "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206";
    await revokeProposerAccess(proposerAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});