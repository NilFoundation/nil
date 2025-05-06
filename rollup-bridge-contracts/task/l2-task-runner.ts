import { task } from "hardhat/config";

// npx hardhat l2-task-runner --networkname local
task("l2-task-runner", "Runs multiple Hardhat tasks")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs, hre) => {
        const networkName = taskArgs.networkname;

        console.log(`Running tasks on network: ${networkName}`);

        console.log("Running clear-l2-deployments...");
        await hre.run("clear-l2-deployments", { networkname: networkName });

        console.log("Running generate-nil-smart-account...");
        await hre.run("generate-nil-smart-account", { networkname: networkName });

        console.log("Running deploy-nil-message-tree...");
        await hre.run("deploy-nil-message-tree", { networkname: networkName });

        console.log("Running deploy-l2-eth-bridge-vault...");
        await hre.run("deploy-l2-eth-bridge-vault", { networkname: networkName });

        console.log("Running deploy-l2-bridge-messenger...");
        await hre.run("deploy-l2-bridge-messenger", { networkname: networkName });

        console.log("Running deploy-l2-eth-bridge...");
        await hre.run("deploy-l2-eth-bridge", { networkname: networkName });

        console.log("Running deploy-l2-enshrined-token-bridge...");
        await hre.run("deploy-l2-enshrined-token-bridge", { networkname: networkName });

        console.log("All tasks completed.");
    });