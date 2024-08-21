const util = require('node:util');
const exec = util.promisify(require('node:child_process').exec);

const SUCCESSFUL_EXECUTION_PATTERN = /Compiler run successful/;

describe.sequential('compilation tests', async () => {
    test.sequential('the CallerAsync contract is compiled successfully', async () => {
        const CALLER_ASYNC_COMPILATION_COMMAND = 'solc -o ./tests/CallerAsync --bin --abi ./tests/CallerAsync.sol --overwrite';
        const { stdout, stderr } = await exec(CALLER_ASYNC_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
    });

    test.sequential('the CallerAsyncBasicPattern contract is compiled successfully', async () => {
        const CALLER_ASYNC_BP_COMPILATION_COMMAND = 'solc -o ./tests/CallerAsyncBasicPattern --bin --abi ./tests/CallerAsyncBasicPattern.sol  --overwrite';
        const { stdout, stderr } = await exec(CALLER_ASYNC_BP_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN);
    });

    test.sequential('the Escrow contract is compiled successfully', async () => {
        const ESCROW_SUCCESSFUL_PATTERN = /Function state mutability can be restricted to pure/;
        const ESCROW_COMPILATION_COMMAND = 'solc -o ./tests/Escrow --bin --abi ./tests/Escrow.sol  --overwrite';
        const { stdout, stderr } = await exec(ESCROW_COMPILATION_COMMAND);
        expect(stderr).toMatch(ESCROW_SUCCESSFUL_PATTERN)
    });

    test.sequential('the Validator contract is compiled successfully', async () => {
        const VALIDATOR_COMPILATION_COMMAND = 'solc -o ./tests/Validator --bin --abi ./tests/Validator.sol  --overwrite';
        const { stdout, stderr } = await exec(VALIDATOR_COMPILATION_COMMAND);
        expect(stdout).toMatch(SUCCESSFUL_EXECUTION_PATTERN)
    });
});





