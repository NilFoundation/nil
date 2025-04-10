import { network } from 'hardhat';
import { authoriseBridges } from './bridges/l1/authorise-bridges-for-messenger';
import { setMessengerInBridges } from './bridges/l1/set-messenger-in-bridges';
import { setMockCounterpartyInBridges } from './bridges/l1/set-mock-counterparty-in-bridges';
import { setRouterInBridge } from './bridges/l1/set-router-in-bridges';
import { setTokenMappings } from './bridges/l1/set-token-mappings';
import { setUserGasFeeInOracle } from './bridges/l1/set-user-gas-fee-in-oracle';

// npx hardhat run scripts/wiring/wiring-master.ts --network geth
export async function wiringMaster() {
    const networkName = network.name;
    await authoriseBridges(networkName);
    await setMessengerInBridges(networkName);
    await setMockCounterpartyInBridges(networkName);
    await setRouterInBridge(networkName);
    await setTokenMappings(networkName);
    await setUserGasFeeInOracle(networkName);
}

async function main() {
    await wiringMaster();
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});
