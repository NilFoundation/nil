import commands from "./commands.mjs";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

export default class TestHelper {
  configFileName;

  constructor({ configFileName }) {
    this.configFileName = configFileName;
  }

  createCLICommandsMap(salt) {
    const result = { ...commands };

    for (const key of Object.keys(commands)) {
      switch (key) {
        case "WALLET_CREATION_COMMAND":
          result[key] = `${commands[key]} --config ${this.configFileName} --salt ${salt}`;
          break;
        default:
          result[key] = `${commands[key]} --config ${this.configFileName}`;
      }
    }

    return result;
  }

  async prepareTestCLI() {
    const testCommands = this.createCLICommandsMap(BigInt(Math.floor(Math.random() * 10000)));

    await exec(testCommands.CONFIG_COMMAND);
    await exec(testCommands.KEYGEN_COMMAND);
    await exec(testCommands.RPC_COMMAND);
    await exec(testCommands.FAUCET_COMMAND);
    await exec(testCommands.COMETA_COMMAND);
    await exec(testCommands.WALLET_CREATION_COMMAND);
  }
}
