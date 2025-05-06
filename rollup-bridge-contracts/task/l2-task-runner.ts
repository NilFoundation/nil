import { task } from "hardhat/config";

// npx hardhat l2-task-runner --networkname local --l1networkname geth
task("l2-task-runner", "Runs multiple Hardhat tasks")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .addParam("l1networkname", "The l1 network to use") // Mandatory parameter
    .setAction(async (taskArgs, hre) => {
        const networkName = taskArgs.networkname;
        const l1NetworkName = taskArgs.l1networkname;

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

        console.log("Running authorise-l2-bridges-in-messenger...");
        await hre.run("authorise-l2-bridges-in-messenger", { networkname: networkName });

        console.log("Running set-eth-bridge-vault-dependencies...");
        await hre.run("set-eth-bridge-vault-dependencies", { networkname: networkName });

        console.log("Running set-eth-bridge-dependencies...");
        await hre.run("set-eth-bridge-dependencies", { networkname: networkName, l1networkname: l1NetworkName });

        console.log("Running set-enshrined-token-bridge-dependencies...");
        await hre.run("set-enshrined-token-bridge-dependencies", { networkname: networkName, l1networkname: l1NetworkName });

        console.log("All tasks completed.");
    });