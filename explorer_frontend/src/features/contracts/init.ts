import { persist } from "effector-storage/local";
import { compileCodeFx } from "../code/model";
import {
  $activeApp,
  $activeAppWithState,
  $activeKeys,
  $assignedAddress,
  $balance,
  $callParams,
  $callResult,
  $contracts,
  $deploymentArgs,
  $error,
  $errors,
  $loading,
  $shardId,
  $state,
  $tokens,
  $txHashes,
  $valueInput,
  assignAdress,
  callFx,
  callMethod,
  choseApp,
  closeApp,
  decrementShardId,
  deploySmartContract,
  deploySmartContractFx,
  fetchBalanceFx,
  incrementShardId,
  sendMethod,
  sendMethodFx,
  setAssignAddress,
  setDeploymentArg,
  setParams,
  setShardId,
  setValueInput,
  toggleActiveKey,
  unlinkApp,
} from "./model";
import { combine, sample } from "effector";
import { isAddress } from "viem";
import { $endpoint, $wallet } from "../account-connector/models/model";
import type { AbiFunction } from "abitype";
import { debug } from "patronum";
import { getTokenAddressBySymbol } from "../currencies";
import type { Token, CometaService, Hex } from "@nilfoundation/niljs";
import { $cometaService } from "../cometa/model";

compileCodeFx.doneData.watch(console.log);

$contracts.on(compileCodeFx.doneData, (_, apps) => apps);
$contracts.reset(compileCodeFx.fail);

persist({
  store: $state,
  key: "contractStates",
});

$activeApp.on(choseApp, (_, { address, bytecode }) => {
  return {
    address,
    bytecode,
  };
});
$activeApp.reset(closeApp);

$error.on(compileCodeFx.failData, (_, error) => `${error}`);
$error.reset(compileCodeFx.doneData);

$deploymentArgs.on(setDeploymentArg, (args, { key, value }) => {
  return {
    ...args,
    [key]: value,
  };
});
$deploymentArgs.reset($activeApp);

$assignedAddress.on(setAssignAddress, (_, address) => address);
$assignedAddress.reset($activeApp);

$state.on(
  sample({
    source: combine($assignedAddress, $activeApp, (address, app) => {
      return {
        address,
        app,
      };
    }),
    clock: assignAdress,
  }),
  (state, { address, app }) => {
    if (!app) {
      return state;
    }
    if (!isAddress(address)) {
      return state;
    }
    const addresses = state[app.bytecode] ? [...state[app.bytecode], address] : [address];
    return {
      ...state,
      [app.bytecode]: addresses,
    };
  },
);

export const $constructor = $activeAppWithState.map((app) => {
  if (!app) {
    return null;
  }
  for (const abi of app.abi) {
    if (abi.type === "constructor") {
      return abi;
    }
  }
  return null;
});

sample({
  source: combine(
    $activeAppWithState,
    $deploymentArgs,
    $wallet,
    $shardId,
    $cometaService,
    (app, args, wallet, shardId, cometaService) => {
      if (!app) {
        return null;
      }
      if (!wallet) {
        return null;
      }
      if (!shardId) {
        return null;
      }
      let abiConstructor = null;
      for (const abi of app.abi) {
        if (abi.type === "constructor") {
          abiConstructor = abi;
          break;
        }
      }
      console.log("abiConstructor", abiConstructor);
      if (!abiConstructor) {
        return {
          app,
          args: [],
          wallet,
          shardId,
        };
      }

      const result: unknown[] = [];
      for (const input of abiConstructor.inputs) {
        let value: unknown;
        switch (true) {
          case input.type === "string": {
            value = value = input.name && input.name in args ? args[input.name] : "";
            break;
          }
          case input.type === "address": {
            value = value = input.name && input.name in args ? args[input.name] : "";
            break;
          }
          case input.type === "bool": {
            value = input.name && input.name in args ? !!args[input.name] : false;
            break;
          }
          case input.type.slice(0, 5) === "bytes": {
            value = input.name && input.name in args ? !!args[input.name] : "";
            break;
          }
          case input.type.slice(0, 3) === "int": {
            value = input.name && input.name in args ? BigInt(args[input.name]) : 0n;
            break;
          }
          default: {
            value = value = input.name && input.name in args ? args[input.name] : "";
            break;
          }
        }
        result.push(value);
      }

      return {
        app,
        args: result,
        wallet,
        shardId,
        cometaService,
      };
    },
  ),
  filter: combine(
    $wallet,
    $activeApp,
    $shardId,
    $cometaService,
    (wallet, app, shardId, cometa) => !!wallet && !!app && shardId !== null && !!cometa,
  ),
  fn: (data) => {
    const { app, args, wallet, shardId, cometaService } = data!;
    return {
      app,
      args,
      wallet,
      shardId: shardId as number, // we have filter
      cometaService: cometaService as CometaService, // we have filter
    };
  },
  clock: deploySmartContract,
  target: deploySmartContractFx,
});

sample({
  source: combine({
    app: $activeApp,
    endpoint: $endpoint,
  }),
  filter: $activeAppWithState.map((app) => !!app?.address),
  clock: choseApp,
  fn: ({ endpoint, app }) => ({ address: app!.address!, endpoint }),
  target: fetchBalanceFx,
});
$state.on(deploySmartContractFx.doneData, (state, { app, address }) => {
  const addresses = state[app] ? [...state[app], address] : [address];
  return {
    ...state,
    [app]: addresses,
  };
});
$state.on(unlinkApp, (state, { app, address }) => {
  const addresses = state[app].filter((addr) => addr !== address);
  return {
    ...state,
    [app]: addresses,
  };
});

$activeApp.on(unlinkApp, () => null);

debug(unlinkApp);

$activeKeys.on(toggleActiveKey, (keys, key) => {
  return {
    ...keys,
    [key]: !keys[key],
  };
});

$activeKeys.reset($activeApp);

$balance.on(fetchBalanceFx.doneData, (_, { balance }) => balance);
$balance.reset($activeApp);

$tokens.on(fetchBalanceFx.doneData, (_, { tokens }) => tokens);
$tokens.reset($activeApp);

sample({
  source: combine({
    activeApp: $activeAppWithState,
    params: $callParams,
  }),
  clock: callMethod,
  filter: $activeAppWithState.map((app) => !!app && !!app.address),
  fn: ({ activeApp, params }, functionName) => {
    let args: unknown[] = [];
    if (!activeApp) {
      args = [];
    } else {
      let abiFunction: AbiFunction | null = null;
      for (const abiField of activeApp.abi) {
        if (abiField.type === "function" && abiField.name === functionName) {
          abiFunction = abiField;
          break;
        }
      }
      if (!abiFunction) {
        args = [];
      } else {
        const callParams = params[functionName];
        for (const input of abiFunction.inputs) {
          if (typeof input.name !== "string") {
            continue;
          }
          const name = input.name;
          args.push(callParams[name] || "");
        }
      }
    }
    return {
      functionName,
      args,
      abi: activeApp?.abi!,
      endpoint: $endpoint.getState(),
      address: activeApp?.address!,
      appName: activeApp?.name,
    };
  },
  target: callFx,
});

$callResult.on(callFx.doneData, (state, { functionName, result }) => {
  return {
    ...state,
    [functionName]: result,
  };
});

sample({
  source: combine({
    activeApp: $activeAppWithState,
    params: $callParams,
    wallet: $wallet,
    valueInput: $valueInput,
  }),
  clock: sendMethod,
  filter: combine(
    $activeAppWithState,
    $wallet,
    (app, wallet) => !!app && !!wallet && !!app.address,
  ),
  fn: ({ activeApp, params, wallet, valueInput }, functionName) => {
    const restParams = params[functionName];

    let args: unknown[] = [];
    if (!activeApp) {
      args = [];
    } else {
      let abiFunction: AbiFunction | null = null;
      for (const abiField of activeApp.abi) {
        if (abiField.type === "function" && abiField.name === functionName) {
          abiFunction = abiField;
          break;
        }
      }
      if (!abiFunction) {
        args = [];
      } else {
        const callParams = restParams;
        for (const input of abiFunction.inputs) {
          if (typeof input.name !== "string") {
            continue;
          }
          const name = input.name;
          args.push(callParams[name] || "");
        }
      }
    }

    const isMainCurrency = valueInput.currency === "NIL";
    const value = isMainCurrency ? valueInput.amount : undefined;
    const tokens: Token[] = isMainCurrency
      ? []
      : [
          {
            id: getTokenAddressBySymbol(valueInput.currency) as Hex,
            amount: BigInt(valueInput.amount),
          },
        ];

    return {
      appName: activeApp?.name,
      functionName,
      args,
      abi: activeApp?.abi!,
      endpoint: $endpoint.getState(),
      address: activeApp?.address!,
      wallet: wallet!,
      ...(value ? { value } : {}),
      ...(tokens.length > 0 ? { tokens } : {}),
    };
  },
  target: sendMethodFx,
});

$loading.on(sendMethodFx, (state, { functionName }) => {
  return {
    ...state,
    [functionName]: true,
  };
});

$loading.on(sendMethodFx.finally, (state, { params: { functionName } }) => {
  return {
    ...state,
    [functionName]: false,
  };
});

$loading.on(callFx, (state, { functionName }) => {
  return {
    ...state,
    [functionName]: true,
  };
});

$loading.on(callFx.finally, (state, { params: { functionName } }) => {
  return {
    ...state,
    [functionName]: false,
  };
});

$loading.reset($activeAppWithState);
$errors.reset($activeAppWithState);
$txHashes.reset($activeAppWithState);
$txHashes.on(sendMethodFx, (state, { functionName }) => {
  return {
    ...state,
    [functionName]: null,
  };
});

$txHashes.on(sendMethodFx.doneData, (state, { functionName, hash }) => {
  return {
    ...state,
    [functionName]: hash,
  };
});

$errors.on(sendMethodFx.fail, (state, { params: { functionName }, error }) => {
  return {
    ...state,
    [functionName]: error.toString(),
  };
});

$callParams.reset($activeAppWithState);

$callParams.on(setParams, (state, { functionName, paramName, value }) => {
  const params = state[functionName] ? { ...state[functionName] } : {};
  params[paramName] = value;

  return {
    ...state,
    [functionName]: params,
  };
});

$shardId.reset($activeAppWithState);
$shardId.on(setShardId, (_, shardId) => shardId);

$activeApp.on(deploySmartContractFx.doneData, (_, { address, app }) => {
  return {
    bytecode: app,
    address,
  };
});

$valueInput.on(setValueInput, (_, value) => value);

$valueInput.reset($activeAppWithState);

$shardId.on(incrementShardId, (shardId, _) => {
  return shardId === null ? 1 : shardId + 1;
});

$shardId.on(decrementShardId, (shardId, _) => {
  return shardId !== null ? Math.max(shardId - 1, 1) : 1;
});
