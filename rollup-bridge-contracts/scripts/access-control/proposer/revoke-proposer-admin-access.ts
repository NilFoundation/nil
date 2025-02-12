import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../../deploy/config/config-helper";
import { getAllProposerAdmins } from "./get-all-proposer-admins";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

// npx hardhat run scripts/access-control/proposer/revoke-proposer-admin-access.ts --network sepolia

// Function to revoke proposerAdmin access
export async function revokeProposerAdminAccess(proposerAdminAddress: string) {

    const networkName = network.name;
    const config = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error("Invalid nilRollupProxy address in config");
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    console.log(`nilRollupProxy on network: ${networkName} at address: ${config.nilRollupProxy} is revoking proposer-admin-access to ${proposerAdminAddress}`);

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(config.nilRollupProxy, abi, signer) as Contract;

    

    // Grant proposer access
    const tx = await nilRollupInstance.revokeProposerAdminRole(proposerAdminAddress);
    await tx.wait();

    console.log(`Proposer-admin access revoked to ${proposerAdminAddress}`);

    const proposerAdmins = await getAllProposerAdmins();
    console.log(`latest list of proposerAdmins are: ${JSON.stringify(proposerAdmins)}`);
}

// Main function to call the revokeProposerAdminAccess function
async function main() {
    const proposerAdminAddress = "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206";
    await revokeProposerAdminAccess(proposerAdminAddress);
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});