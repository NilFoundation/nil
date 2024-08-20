import { NODE_MODULES } from './globals';

const util = require('node:util');
const exec = util.promisify(require('node:child_process').exec);

const SUCCESSFUL_EXECUTION_PATTERN = /Compiler run successful/;

describe.sequential('Receiver tests', async () => {
  test.sequential('the Receiver contract is compiled successfully', async () => {
    const RECEIVER_COMPILATION_COMMAND = `solc -o ./tests/Receiver --bin --abi ./tests/Receiver.sol --overwrite ${NODE_MODULES}`;

    const { stdout, stderr } = await exec(RECEIVER_COMPILATION_COMMAND);
    expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
  });
});