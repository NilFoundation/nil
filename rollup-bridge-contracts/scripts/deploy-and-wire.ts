import { run } from "hardhat";
import { wiringMaster } from "./wiring/wiring-master";
import { clearDeployments } from "./wiring/clear-deployments";
import { setDeployerConfig } from "./wiring/set-deployer-config";

// npx hardhat run scripts/deploy-and-wire.ts --network geth
async function main() {
    const networkName = (await import("hardhat")).network.name;

    console.log(`Starting unified deployment and wiring process on network: ${networkName}`);

    console.log("Running clearDeployments...");
    await clearDeployments(networkName);

    console.log("Running setDeployerConfig...");
    await setDeployerConfig(networkName);

    console.log("Running DeployL1Mock...");
    await run("deploy", { tags: "DeployL1Mock" });

    console.log("Running DeployL1Master...");
    await run("deploy", { tags: "DeployL1Master" });

    console.log("Running wiring-master...");
    await wiringMaster(networkName);

    console.log("Unified deployment and wiring process completed successfully!");
}

main().catch((error) => {
    console.error("Error during unified deployment and wiring process:", error);
    process.exit(1);
});
