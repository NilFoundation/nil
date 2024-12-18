const path = require('node:path');
const fs = require('node:fs');
const solc = require('solc');
const { rimrafSync } = require('rimraf');
const rollup = require('@rollup/wasm-node').rollup;
const typescript = require("@rollup/plugin-typescript");

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
const esmObject = {}
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
    const abiBuildPath = path.join(artifactsDir, `${contractName}.abi.json`);
    const binBuildPath = path.join(artifactsDir, `${contractName}.bin.json`);

    esmObject[contractName] = {
      abi: contractOutput.abi,
      bytecode: contractOutput.evm.bytecode.object,
    };

    fs.writeFileSync(contractBuildPath, JSON.stringify(contractOutput, null, 2));
    fs.writeFileSync(abiBuildPath, JSON.stringify(contractOutput.abi));
    fs.writeFileSync(binBuildPath, JSON.stringify(contractOutput.evm.bytecode.object));
  }
}

const esmBuildPath = path.join(artifactsDir, 'index.ts');
let esmContent = '';
for (const contractName in esmObject) {
  esmContent += `export const ${contractName} = ${JSON.stringify(esmObject[contractName], null, 2)} as const;\n`;
}
fs.writeFileSync(esmBuildPath, esmContent);

const outputOptionsList = [{
  file: path.join(artifactsDir, 'index.cjs.js'),
  format: 'cjs',
  name: 'index',
},
  {
    file: path.join(artifactsDir, 'index.esm.js'),
    format: 'esm',
    name: 'index',
  }];

async function generateOutputs(bundle) {
  for (const outputOptions of outputOptionsList) {
    await bundle.write(outputOptions);
  }
}

rollup({
  input: esmBuildPath,
  output: [{
    file: path.join(artifactsDir, 'index.cjs.js'),
    format: 'cjs',
  },
  {
    file: path.join(artifactsDir, 'index.esm.js'),
    format: 'esm',
  }],
  plugins: [typescript({
    target: "ES2023",
    include: [esmBuildPath],
    compilerOptions: {

      declaration: true,
      declarationDir: artifactsDir,
      },
  })],
})
.then(bundle => generateOutputs(bundle))
.then(() => {
  fs.unlinkSync(esmBuildPath);
  console.log('Compilation successful');
})

