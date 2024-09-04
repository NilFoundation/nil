const path = require('node:path');
const fs = require('node:fs');
const solc = require('solc');
const { rimrafSync } = require('rimraf');

const scriptDir = __dirname;
const contractsDir = path.join(scriptDir, '../contracts');
const artifactsDir = path.join(scriptDir, '../artifacts');

// clear contents of the build directories
rimrafSync(artifactsDir);
fs.mkdirSync(artifactsDir);

// get contracts contents
const contentsMap = new Map();
const contents = fs.readdirSync(contractsDir);
const contracts = contents.filter((content) => content.endsWith('.sol'));

for (const contract of contracts) {
  const contractPath = path.join(contractsDir, contract);
  const contractContent = fs.readFileSync(contractPath).toString();
  contentsMap.set(contract, contractContent);
}

// compile the smart contracts
for (const contract of contracts) {
  console.log(`Compiling ${contract}...`);
  const contractContent = contentsMap.get(contract);

  const input = {
    language: 'Solidity',
    sources: {
      [contract]: {
        content: contractContent,
      }
    },
    settings: {
      outputSelection: {
        '*': {
          '*': ['*'],
        },
      },
    },
  };

  const findImports = (p) => {
    for (const entry of contentsMap.entries()) {
      const [contract, content] = entry;
      if (p === contract) {
        return { contents: content };
      }
    }

    return { error: 'File not found' };
  }

  const output = JSON.parse(solc.compile(JSON.stringify(input), {
    import: findImports,
  }));

  const blacklistedContracts = ['__Precompile__'];

  for (const contractName in output.contracts[contract]) {
    if (blacklistedContracts.includes(contractName)) {
      continue;
    }

    const contractOutput = output.contracts[contract][contractName];
    const contractBuildPath = path.join(artifactsDir, `${contractName}.json`);
    fs.writeFileSync(contractBuildPath, JSON.stringify(contractOutput, null, 2));
  }
};
