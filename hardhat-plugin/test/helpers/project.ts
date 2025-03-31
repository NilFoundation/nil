import fs from "node:fs";

const configPath = "./config.ini";
const envPath = "./.env";
const envFileContent = `
NIL_RPC_ENDPOINT: "http://127.0.0.1:8529"
COMETA_ENDPOINT: "http://127.0.0.1:8528"
FAUCET_ENDPOINT: "http://127.0.0.1:8527"
NIL_CONFIG_INI: "${configPath}"
HARDHAT_IGNITION_CONFIRM_DEPLOYMENT: "false"
HARDHAT_IGNITION_CONFIRM_RESET: "false"
`;

export function useHardhatProject(projectName: string) {
  const previousCwd = process.cwd();
  before("Loading Hardhat Runtime Environment", async function () {
    process.chdir(`${__dirname}/../projects/${projectName}`);
    fs.writeFileSync(envPath, envFileContent);
    this.hre = require("hardhat");
  });

  after("Resetting Hardhat's context, CWD and deleting context fields", function () {
    fs.unlinkSync(envPath);
    fs.unlinkSync(configPath);
    process.chdir(previousCwd);
    (this as any).hre = undefined;
  });
}
