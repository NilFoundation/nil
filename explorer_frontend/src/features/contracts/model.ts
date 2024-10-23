import { combine, createEffect, createEvent, createStore } from "effector";
import type { App } from "../../types";
import type { Abi, Address } from "abitype";
import {
  HttpTransport,
  PublicClient,
  convertEthToWei,
  waitTillCompleted,
  type WalletV1,
  getShardIdFromAddress,
  type Token,
} from "@nilfoundation/niljs";

export type DeployedApp = App & {
  address: Address;
};

export const $contracts = createStore<App[]>([]);
export const $state = createStore<{ [code: string]: Address[] }>({});
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

export const $contractWithState = combine($contracts, $state, (contracts, state) => {
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

export const $error = createStore<string | null>(null);

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
export const $assignedAddress = createStore<string>("");
export const setAssignAddress = createEvent<string>();

export const $shardId = createStore<number>(1);

export const setShardId = createEvent<number>();

export const deploySmartContract = createEvent();
export const deploySmartContractFx = createEffect<
  {
    app: App;
    args: unknown[];
    shardId: number;
    wallet: WalletV1;
  },
  {
    address: `0x${string}`;
    app: `0x${string}`;
    name: string;
  }
>(async ({ app, args, wallet, shardId }) => {
  const salt = BigInt(Math.floor(Math.random() * 10000000000000000));

  const { hash, address } = await wallet.deployContract({
    bytecode: app.bytecode,
    abi: app.abi,
    args,
    salt,
    shardId,
    feeCredit: convertEthToWei(0.00001),
  });

  await waitTillCompleted(wallet.client, wallet.shardId, hash);

  return {
    address,
    app: app.bytecode,
    name: app.name,
  };
});

export const assignAdress = createEvent();

export const $balance = createStore<bigint>(0n);
export const $tokens = createStore<Record<`0x${string}`, bigint>>({});

export const fetchBalanceFx = createEffect<
  {
    address: `0x${string}`;
    endpoint: string;
  },
  {
    tokens: Record<`0x${string}`, bigint>;
    balance: bigint;
  }
>(async ({ address, endpoint }) => {
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint }),
  });
  const [tokens, balance] = await Promise.all([
    client.getCurrencies(address, "latest"),
    client.getBalance(address, "latest"),
  ]);
  return {
    tokens,
    balance,
  };
});

export const $managementKey = createStore<string>("read");
export const setManagementPage = createEvent<string>();

export const $activeKeys = createStore<Record<string, boolean>>({});

export const toggleActiveKey = createEvent<string>();

export const $callParams = createStore<Record<string, Record<string, unknown>>>({});

export const setParams = createEvent<{
  functionName: string;
  paramName: string;
  value: unknown;
}>();

export const $callResult = createStore<Record<string, unknown>>({});

export const callFx = createEffect<
  {
    functionName: string;
    abi: Abi;
    args: unknown[];
    endpoint: string;
    address: `0x${string}`;
  },
  {
    functionName: string;
    result: unknown;
  }
>(async ({ functionName, args, endpoint, abi, address }) => {
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint }),
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

  console.log("Call result", data);

  return {
    functionName,
    result: data.decodedData,
  };
});

export const callMethod = createEvent<string>();

export const sendMethodFx = createEffect<
  {
    abi: Abi;
    functionName: string;
    args: unknown[];
    wallet: WalletV1;
    address: `0x${string}`;
    value?: string;
    tokens?: Token[];
  },
  { functionName: string; hash: string }
>(async ({ abi, functionName, args, wallet, address, value, tokens }) => {
  const hash = await wallet.sendMessage({
    abi,
    functionName,
    args,
    to: address,
    feeCredit: convertEthToWei(0.001),
    value: value ? convertEthToWei(Number(value)) : undefined,
    tokens: tokens,
  });

  await waitTillCompleted(wallet.client, getShardIdFromAddress(wallet.getAddressHex()), hash);

  return {
    functionName,
    hash,
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

export const $valueInput = createStore<{
  currency: string;
  amount: string;
}>({
  currency: "NIL",
  amount: "0",
});

export const setValueInput = createEvent<{
  currency: string;
  amount: string;
}>();
