import { ethers, network } from 'hardhat';
import { Contract } from 'ethers';
import * as fs from 'fs';
import * as path from 'path';
import {
    isValidAddress,
    loadConfig,
} from '../../../deploy/config/config-helper';
import { hasRole } from '../has-a-role';
import { DEFAULT_ADMIN_ROLE } from '../../utils/roles';

// Function to check if an address is a Admin
export async function isAnAdmin(account: string): Promise<Boolean> {
    const isAnAdminResponse = await hasRole(DEFAULT_ADMIN_ROLE, account);

    // Convert the response to a boolean
    const isAnAdminIndicator = Boolean(isAnAdminResponse);

    return isAnAdminIndicator;
}
