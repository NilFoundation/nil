import { PROPOSER_ROLE } from '../../utils/roles';
import { getRoleMembers } from '../get-role-members';

// npx hardhat run scripts/access-control/proposer/get-all-proposers.ts --network sepolia
export async function getAllProposers() {
    const proposers = await getRoleMembers(PROPOSER_ROLE);
    return proposers;
}
