import { DEFAULT_ADMIN_ROLE, PROPOSER_ROLE } from "../../utils/roles";
import { getRoleMembers } from "../get-role-members";

// npx hardhat run scripts/access-control/admin/get-all-admins.ts --network sepolia
export async function getAllAdmins() {
    const admins = await getRoleMembers(DEFAULT_ADMIN_ROLE);
    console.log(`admins are: ${JSON.stringify(admins)}`);
}

async function main() {
    await getAllAdmins();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});