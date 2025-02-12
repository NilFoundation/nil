import { PROPOSER_ROLE_ADMIN } from "../../utils/roles";
import { getRoleMembers } from "../get-role-members";

// npx hardhat run scripts/access-control/proposer/get-all-proposer-admins.ts --network sepolia
export async function getAllProposerAdmins() {
    const proposerAdmins = await getRoleMembers(PROPOSER_ROLE_ADMIN);
    console.log(`proposerAdmins are: ${JSON.stringify(proposerAdmins)}`);
}

// Main function to call the getAllProposerAdmins function
async function main() {
    await getAllProposerAdmins();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});