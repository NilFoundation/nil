import * as esbuild from "esbuild-wasm";
import type { App } from "../../features/code/types";
import { contractResolverPlugin } from "../contractResolver/contractResolverPlugin";
import { cdnResolverPlugin } from "./cdnResolverPlugin";
let initilized = false;
const esbuildVersion = "0.25.2";

function splitImports(input: string) {
  const importRegex = /^(import\s+(?:[^'"`]+from\s+)?['"`][^'"`]+['"`];?\s*)$/gm;

  const importsArray = [];
  let match: RegExpExecArray | null = null;

  // biome-ignore lint/suspicious/noAssignInExpressions: <explanation>
  while ((match = importRegex.exec(input)) !== null) {
    importsArray.push(match[1]);
  }

  const code = input.replace(importRegex, "").trim();
  const imports = importsArray.join("\n");

  return { imports, code };
}

const injectContractsCode = (contracts: App[], rpcUrl: string) => {
  const repository: Record<string, App> = contracts.reduce(
    (acc, contract) => {
      acc[contract.name] = contract;
      return acc;
    },
    {} as Record<string, App>,
  );

  const repoString = JSON.stringify(repository);
  const code = `
  import prettyFormat from "pretty-format";
  
  import {getContract, PublicClient, HttpTransport, Transaction, hexToBytes} from '@nilfoundation/niljs';
  (function() {
    const originalLog = console.log;
    const originalWarn = console.warn;
    console.log = function(...args) {
      self.postMessage({ type: 'log', args: prettyFormat(args) });
      originalLog.apply(console, args);
    };
    console.warn = function(...args) {
      self.postMessage({ type: 'warn', args: prettyFormat(args) });
      originalWarn.apply(console, args);
    };
  })();
  const ___client = new PublicClient({
    transport: new HttpTransport({
      endpoint: '${rpcUrl}',
      fetcher: (...args) => {return fetch(...args);},
    }),
  });
  const ___requestMap = new Map();
  let ___requestId = 0;
  const ___smartAccount = {
    sendTransaction(params) {
      ___requestId++;
      let curRequestId = ___requestId;
      self.postMessage({
          type: 'sendTransaction',
          params,
          id: curRequestId,
        });
      return new Promise((resolve, reject) => {
        ___requestMap.set(curRequestId, {
            resolve,
            reject
          });
      });
    }
  };
  addEventListener("message", (event) => {
    const { data } = event;
    if (data.type === 'sendTransactionResult') {
      const { result, id, status } = data;
      const { resolve, reject} = ___requestMap.get(id);
      if (status === 'ok') {
        const hash = result;
        resolve(new Transaction(hexToBytes(hash), ___client));
      } else {
        reject(new Error('Transaction failed: ' + result));
      }
      ___requestMap.delete(id);
    }
  });
const getContractAt = async (contractName: string, address: string) => {
  const repository = ${repoString};

  if (!repository[contractName]) {
    throw new Error(\`Contract \${contractName} not found\`);
  }

  return getContract({
    abi: repository[contractName].abi,
    bytecode: repository[contractName].bytecode,
    smartAccount: ___smartAccount,
    client: ___client,
    address,
  })
};`;
  return code;
};

const virualEntryFileName = "index.ts";
const virtualEntryPlugin = (code: string) => ({
  name: "virtual-entry",
  setup(build: esbuild.PluginBuild) {
    build.onResolve({ filter: /.*/ }, (args) => {
      if (args.path === virualEntryFileName) {
        return { path: args.path, namespace: "a" };
      }
    });

    build.onLoad({ filter: /.*/, namespace: "a" }, () => {
      return {
        contents: code,
        loader: "tsx",
      };
    });
  },
});

export const bundle = async (
  code: string,
  contracts: App[],
  rpcUrl: string,
  injections: esbuild.Plugin[] = [],
): Promise<string> => {
  const { code: splittedCode, imports: splittedImports } = splitImports(code);
  if (!initilized) {
    initilized = true;
    await esbuild.initialize({
      wasmURL: `https://unpkg.com/esbuild-wasm@${esbuildVersion}/esbuild.wasm`,
    });

    console.log("esbuild initialized");
  }

  const build = await esbuild.build({
    entryPoints: [virualEntryFileName],
    bundle: true,
    write: false,
    plugins: [
      virtualEntryPlugin(
        `${splittedImports}${injectContractsCode(contracts, rpcUrl)}async function ___nil_something(){${splittedCode}}; ___nil_something();`,
      ),
      contractResolverPlugin(contracts),
      cdnResolverPlugin(),
      ...injections,
    ],
    format: "esm",
    target: ["es2020"],
    minify: false,
    sourcemap: "inline",
  });

  if (build.errors.length > 0) {
    const firstError = build.errors[0];
    const error: Error & { locations?: { text: string; line: number | null }[] } = new Error(
      `Error: ${firstError.text} \nLocation: ${firstError.location?.file}:${firstError.location?.line}:${firstError.location?.column}`,
    );
    error.locations = build.errors.map((error) => ({
      text: error.text,
      line: error.location?.line || null,
    }));
    throw error;
  }
  if (build.warnings.length > 0) {
    console.warn("Warnings:", build.warnings);
  }
  if (build.outputFiles.length === 0) {
    throw new Error("No output files found");
  }

  return build.outputFiles[0].text;
};
