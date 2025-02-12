import { PROPOSER_ROLE } from "../../utils/roles";
import { getRoleMembers } from "../get-role-members";

// npx hardhat run scripts/access-control/proposer/get-all-proposers.ts --network sepolia
export async function getAllProposers() {
    const proposers = await getRoleMembers(PROPOSER_ROLE);
    return proposers;
}

// Main function to call the getAllProposers function
// async function main() {
//     await getAllProposers();
// }

// main().catch((error) => {
//     console.error(error);
//     process.exit(1);
// });