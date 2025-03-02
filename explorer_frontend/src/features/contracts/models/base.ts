import {
  type CometaService,
  type Hex,
  HttpTransport,
  PublicClient,
  type SmartAccountV1,
  type Token,
  bytesToHex,
  convertEthToWei,
  isHexString,
  removeHexPrefix,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import type { Abi, Address } from "abitype";
import { combine, createEffect, createEvent, createStore } from "effector";
import { ethers } from "ethers";
import type { App } from "../../code/types";
import { createCompileInput } from "../../shared/utils/solidityCompiler/helper";
import { ActiveComponent } from "../components/Deploy/ActiveComponent";
import type { CallParams } from "../types";

export type DeployedApp = App & {
  address: Address;
};

export const $contracts = createStore<App[]>([]);
export const $deployedContracts = createStore<{ [code: string]: Address[] }>({});
export const $activeApp = createStore<{
  bytecode: `0x${string}`;
  address?: Address;
} | null>(null);

export const choseApp = createEvent<{
  bytecode: `0x${string}`;
  address?: Address;
}>();
export const closeApp = createEvent();

export const resetApps = createEvent();

export const $contractWithState = combine($contracts, $deployedContracts, (contracts, state) => {
  const contractsWithAddress: (App & { address?: Address })[] = [];
  for (const contract of contracts) {
    if (state[contract.bytecode]) {
      for (const address of state[contract.bytecode]) {
        contractsWithAddress.push({
          ...contract,
          address,
        });
      }
    }
  }
  return contractsWithAddress;
});

export const $activeAppWithState = combine($activeApp, $contracts, (activeApp, contracts) => {
  if (activeApp === null) {
    return null;
  }
  const { bytecode, address } = activeApp;
  const contract = contracts.find((contract) => contract.bytecode === bytecode) || null;

  if (!contract) {
    return null;
  }

  return {
    ...contract,
    address,
  };
});

export const $deploymentArgs = createStore<Record<string, string | boolean>>({});
export const setDeploymentArg = createEvent<{
  key: string;
  value: string | boolean;
}>();
export const $importedAddress = createStore<string>("");
export const setAssignAddress = createEvent<string>();

export const $shardId = createStore<number | null>(null);

export const setShardId = createEvent<number | null>();
export const setRandomShardId = createEvent();
export const incrementShardId = createEvent("increment");
export const decrementShardId = createEvent("decrement");
export const triggerShardIdValidation = createEvent();

export const $deploySmartContractError = createStore<string | null>(null);

export const deploySmartContract = createEvent();
export const deploySmartContractFx = createEffect<
  {
    app: App;
    args: unknown[];
    shardId: number;
    smartAccount: SmartAccountV1;
  },
  {
    address: Hex;
    app: Hex;
    name: string;
    deployedFrom?: Hex;
    txHash: Hex;
  }
>(async ({ app, args, smartAccount, shardId }) => {
  const salt = BigInt(Math.floor(Math.random() * 10000000000000000));

  const { hash, address } = await smartAccount.deployContract({
    bytecode: app.bytecode,
    abi: app.abi,
    args,
    salt,
    shardId,
    feeCredit: convertEthToWei(0.00001),
  });

  await waitTillCompleted(smartAccount.client, hash);

  return {
    address,
    app: app.bytecode,
    name: app.name,
    deployedFrom: smartAccount.address,
    txHash: hash,
  };
});

export const registerContractInCometaFx = createEffect<
  {
    name: string;
    app: App;
    address: Hex;
    cometaService: CometaService;
    solidityVersion: string;
  },
  void
>(async ({ name, app, address, cometaService, solidityVersion }) => {
  const result = createCompileInput(app.sourcecode);

  const refinedSolidityVersion = solidityVersion.match(/\d+\.\d+\.\d+/)?.[0] || "";

  const refinedResult = {
    ...result,
    contractName: `Compiled_Contracts:${name}`,
    compilerVersion: refinedSolidityVersion,
  };

  await cometaService.registerContract(JSON.stringify(refinedResult), address);
});

export const $importedSmartContractAddress = createStore<Hex>("" as Hex);
export const $importedSmartContractAddressIsValid = createStore<boolean>(true);
export const $importedSmartContractAddressError = createStore<string | null>(null);
export const setImportedSmartContractAddress = createEvent<Hex>();
export const setImportedSmartContractAddressError = createEvent<string | null>();
export const setImportedSmartContractAddressIsValid = createEvent<boolean>();
export const importSmartContract = createEvent();

export const validateSmartContractAddressFx = createEffect<
  {
    address: Hex;
    deployedContracts: { [code: string]: Address[] };
  },
  boolean
>(async ({ address, deployedContracts }) => {
  const validations = [
    {
      fn: (addr: string) => addr.length === 42,
      err: "Address should be 42 characters long",
    },
    {
      fn: (addr: string) => addr.startsWith("0x"),
      err: "Address should start with 0x",
    },
    {
      fn: (addr: string) => isHexString(addr),
      err: "Address should contain only hex characters",
    },
    {
      fn: (addr: string) => {
        const existingAddresses = Object.values(deployedContracts).flat();
        return !existingAddresses.includes(addr as Hex);
      },
      err: "Contract with address already exists",
    },
  ];

  for (const validation of validations) {
    if (!validation.fn(address)) {
      setImportedSmartContractAddressIsValid(false);
      setImportedSmartContractAddressError(validation.err);
      throw new Error(validation.err);
    }
  }

  setImportedSmartContractAddressIsValid(true);
  setImportedSmartContractAddressError(null);
  return true;
});

export const importSmartContractFx = createEffect<
  {
    app: App;
    smartAccount: SmartAccountV1;
    importedSmartContractAddress: Hex;
  },
  {
    importedSmartContractAddress: Hex;
    app: Hex;
  }
>(async ({ app, smartAccount, importedSmartContractAddress }) => {
  const source = removeHexPrefix(
    bytesToHex(await smartAccount.client.getCode(importedSmartContractAddress, "latest")),
  );

  if (source === "") {
    setImportedSmartContractAddressError("Contract with this address does not exist");
    throw new Error(`Contract with address ${importedSmartContractAddress} does not exist`);
  }

  if (!app.bytecode.includes(source)) {
    setImportedSmartContractAddressError(
      `Interface of the importing contract is not compatible with ${app.name}`,
    );
    throw new Error(
      `Interface of the contract with address ${importedSmartContractAddress} is not compatible with ${app.name}`,
    );
  }

  return {
    importedSmartContractAddress,
    app: app.bytecode,
  };
});

export const $balance = createStore<bigint>(0n);
export const $tokens = createStore<Record<`0x${string}`, bigint>>({});

export const fetchBalanceFx = createEffect<
  {
    address: `0x${string}`;
    rpcUrl: string;
  },
  {
    tokens: Record<`0x${string}`, bigint>;
    balance: bigint;
  }
>(async ({ address, rpcUrl }) => {
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint: rpcUrl }),
  });
  const [tokens, balance] = await Promise.all([
    client.getTokens(address, "latest"),
    client.getBalance(address, "latest"),
  ]);
  return {
    tokens,
    balance,
  };
});

export const $activeKeys = createStore<Record<string, boolean>>({});

export const toggleActiveKey = createEvent<string>();

export const $callParams = createStore<CallParams>({});

export const setParams = createEvent<{
  functionName: string;
  paramName: string;
  value:
    | string
    | boolean
    | {
        type: string;
        value: string | boolean;
      }[];
  type: string;
}>();

export const $callResult = createStore<Record<string, unknown>>({});

export const callFx = createEffect<
  {
    appName?: string;
    functionName: string;
    abi: Abi;
    args: unknown[];
    rpcUrl: string;
    address: `0x${string}`;
  },
  {
    functionName: string;
    result: unknown;
    appName?: string;
  }
>(async ({ functionName, args, rpcUrl, abi, address, appName }) => {
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint: rpcUrl }),
  });

  const data = await client.call(
    {
      to: address,
      abi,
      args,
      functionName,
      feeCredit: convertEthToWei(0.001),
    },
    "latest",
  );

  return {
    functionName,
    result: data.decodedData,
    appName,
  };
});

export const callMethod = createEvent<string>();

export const sendMethodFx = createEffect<
  {
    appName?: string;
    abi: Abi;
    functionName: string;
    args: unknown[];
    smartAccount: SmartAccountV1;
    address: `0x${string}`;
    value?: string;
    tokens?: Token[];
  },
  {
    functionName: string;
    hash: Hex;
    sendFrom: Hex;
    appName?: string;
    txLogs: string[];
  }
>(async ({ abi, functionName, args, smartAccount, address, value, tokens, appName }) => {
  const hash = await smartAccount.sendTransaction({
    abi,
    functionName,
    args,
    to: address,
    feeCredit: convertEthToWei(0.001),
    value: value ? convertEthToWei(Number(value)) : undefined,
    tokens: tokens,
  });

  await waitTillCompleted(smartAccount.client, hash);
  const contractIface = new ethers.Interface(abi);
  const receipts = await smartAccount.client.getTransactionReceiptByHash(hash);
  const logs = receipts
    ? [
        ...(receipts.outputReceipts?.flatMap((receipt) => {
          return receipt ? receipt.logs : [];
        }) ?? []),
        ...receipts.logs,
      ]
    : [];
  const txLogs = logs
    .map((log): string | null => {
      const parsedLog = contractIface.parseLog(log);
      if (parsedLog == null) {
        return null;
      }
      return `${parsedLog?.name} with args [${parsedLog?.args.join(", ")}]`;
    })
    .filter((log): log is string => log !== null);

  return {
    functionName,
    hash,
    sendFrom: smartAccount.address,
    appName,
    txLogs,
  };
});

export const sendMethod = createEvent<string>();

export const $loading = createStore<Record<string, boolean>>({});
export const $errors = createStore<Record<string, string | null>>({});
export const $txHashes = createStore<Record<string, string | null>>({});

export const unlinkApp = createEvent<{
  app: `0x${string}`;
  address: `0x${string}`;
}>();

export const $valueInputs = createStore<
  Array<{
    values: {
      token: string;
      amount: string;
    }[];
    functionName: string;
  }>
>([]);

export const setValueInput = createEvent<{
  functionName: string;
  index: number;
  token: string;
  amount: string;
}>();
export const addValueInput = createEvent<{
  functionName: string;
  availableTokens: string[];
}>();
export const removeValueInput = createEvent<{
  functionName: string;
  index: number;
}>();

export const $activeComponent = createStore<ActiveComponent>(ActiveComponent.Deploy);
export const setActiveComponent = createEvent<ActiveComponent>();

export const $shardIdIsValid = createStore<boolean>(true);
