import { RECEIVER_COMPILATION_COMMAND } from "./compilationCommands";
import { SUCCESSFUL_EXECUTION_PATTERN } from "./patterns";
const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

describe.sequential("Receiver tests", async () => {
  test.sequential("the Receiver contract is compiled successfully", async () => {
    const { stdout, stderr } = await exec(RECEIVER_COMPILATION_COMMAND);
    expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
  });
});
