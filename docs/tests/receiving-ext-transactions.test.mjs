import { RECEIVER_COMPILATION_COMMAND } from "./compilationCommands";
import { SUCCESSFUL_EXECUTION_PATTERN } from "./patterns";
const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

describe.sequential("Receiver tests", async () => {
  test.sequential("the Receiver contract is compiled successfully", async () => {
    console.log(RECEIVER_COMPILATION_COMMAND);
    const { stdout, stderr } = await exec(RECEIVER_COMPILATION_COMMAND);
    console.log("stdout!!!!!: ", stdout);
    expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
  });
});
