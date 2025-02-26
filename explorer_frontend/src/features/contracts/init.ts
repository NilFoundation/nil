import type { CometaService, Hex, SmartAccountV1, Token } from "@nilfoundation/niljs";
import type { AbiFunction } from "abitype";
import { combine, merge, sample } from "effector";
import { persist } from "effector-storage/local";
import { debug } from "patronum";
import type { App } from "../../types";
import { $rpcUrl, $smartAccount } from "../account-connector/model";
import { $solidityVersion, compileCodeFx, loadedPlaygroundPage } from "../code/model";
import { $cometaService } from "../cometa/model";
import { getTokenAddressBySymbol } from "../tokens";
import {
  $activeApp,
  $activeAppWithState,
  $activeComponent,
  $activeKeys,
  $balance,
  $callParams,
  $callResult,
  $contracts,
  $deploySmartContractError,
  $deployedContracts,
  $deploymentArgs,
  $errors,
  $importedAddress,
  $importedSmartContractAddress,
  $importedSmartContractAddressError,
  $importedSmartContractAddressIsValid,
  $loading,
  $shardId,
  $shardIdIsValid,
  $tokens,
  $txHashes,
  $valueInputs,
  addValueInput,
  callFx,
  callMethod,
  choseApp,
  closeApp,
  decrementShardId,
  deploySmartContract,
  deploySmartContractFx,
  fetchBalanceFx,
  importSmartContract,
  importSmartContractFx,
  incrementShardId,
  registerContractInCometaFx,
  removeValueInput,
  sendMethod,
  sendMethodFx,
  setActiveComponent,
  setAssignAddress,
  setDeploymentArg,
  setImportedSmartContractAddress,
  setImportedSmartContractAddressError,
  setImportedSmartContractAddressIsValid,
  setParams,
  setRandomShardId,
  setShardId,
  setValueInput,
  toggleActiveKey,
  triggerShardIdValidation,
  unlinkApp,
  validateSmartContractAddressFx,
} from "./models/base";
import { exportApp, exportAppFx } from "./models/exportApp";
import { $shardsAmount, getShardsAmountFx } from "./models/shardsAmount";

compileCodeFx.doneData.watch(console.log);

$contracts.on(compileCodeFx.doneData, (_, { apps }) => apps);
$contracts.reset(compileCodeFx.fail);

persist({
  store: $deployedContracts,
  key: "contractStates",
});

$activeApp.on(choseApp, (_, { address, bytecode }) => {
  return {
    address,
    bytecode,
  };
});
$activeApp.reset(closeApp);

$deploymentArgs.on(setDeploymentArg, (args, { key, value }) => {
  return {
    ...args,
    [key]: value,
  };
});
$deploymentArgs.reset($activeApp);

$importedAddress.on(setAssignAddress, (_, address) => address);
$importedAddress.reset($activeApp);

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
    $smartAccount,
    $shardId,
    (app, args, smartAccount, shardId) => {
      if (!app) {
        return null;
      }
      if (!smartAccount) {
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

      if (!abiConstructor) {
        return {
          app,
          args: [],
          smartAccount,
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
            value = input.name && input.name in args ? args[input.name] : "";
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
        smartAccount,
        shardId,
      };
    },
  ),
  filter: combine(
    $smartAccount,
    $activeApp,
    $shardId,
    (smartAccount, app, shardId) => !!smartAccount && !!app && shardId !== null,
  ),
  fn: (data) => {
    const { app, args, smartAccount, shardId } = data!;
    return {
      app,
      args,
      smartAccount,
      shardId: shardId as number, // we have filter
    };
  },
  clock: deploySmartContract,
  target: deploySmartContractFx,
});

$deploySmartContractError.reset($activeApp);
$deploySmartContractError.reset(deploySmartContract);
$deploySmartContractError.on(deploySmartContractFx.failData, (_, error) => String(error));

$importedSmartContractAddress.on(setImportedSmartContractAddress, (_, address) => address);
$importedSmartContractAddress.reset($activeApp);

$importedSmartContractAddressIsValid.reset($activeApp);
$importedSmartContractAddressIsValid.reset(setImportedSmartContractAddress);
$importedSmartContractAddressIsValid.on(
  setImportedSmartContractAddressIsValid,
  (_, isValid) => isValid,
);

$importedSmartContractAddressError.reset($activeApp);
$importedSmartContractAddressError.reset(setImportedSmartContractAddress);
$importedSmartContractAddressError.on(setImportedSmartContractAddressError, (_, err) => {
  return err;
});

sample({
  source: combine(
    $activeAppWithState,
    $smartAccount,
    $importedSmartContractAddress,
    (app, smartAccount, importedSmartContractAddress) => {
      return {
        app,
        smartAccount,
        importedSmartContractAddress,
      };
    },
  ),
  filter: combine(
    $smartAccount,
    $activeApp,
    $importedSmartContractAddress,
    (smartAccount, app, importedSmartContractAddress) =>
      !!smartAccount && !!app && !!importedSmartContractAddress,
  ),
  fn: (data) => {
    const { app, smartAccount, importedSmartContractAddress } = data!;
    return {
      app: app as App,
      smartAccount: smartAccount as SmartAccountV1,
      importedSmartContractAddress: importedSmartContractAddress as Hex,
    };
  },
  clock: validateSmartContractAddressFx.doneData,
  target: importSmartContractFx,
});

sample({
  clock: importSmartContract,
  source: combine({
    address: $importedSmartContractAddress,
    deployedContracts: $deployedContracts,
  }),
  target: validateSmartContractAddressFx,
});

sample({
  source: combine({
    app: $activeApp,
    rpcUrl: $rpcUrl,
  }),
  filter: $activeAppWithState.map((app) => !!app?.address),
  clock: choseApp,
  fn: ({ rpcUrl, app }) => ({ address: app?.address!, rpcUrl }),
  target: fetchBalanceFx,
});

$deployedContracts.on(deploySmartContractFx.doneData, (state, { app, address }) => {
  const addresses = state[app] ? [...state[app], address] : [address];
  return {
    ...state,
    [app]: addresses,
  };
});

$deployedContracts.on(
  importSmartContractFx.doneData,
  (state, { app, importedSmartContractAddress }) => {
    const addresses = state[app]
      ? [...state[app], importedSmartContractAddress]
      : [importedSmartContractAddress];
    return {
      ...state,
      [app]: addresses,
    };
  },
);

$deployedContracts.on(unlinkApp, (state, { app, address }) => {
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
          if (input.name === "") {
            for (const key in callParams) {
              args.push(callParams[key] || "");
            }
          } else {
            const name = input.name;
            args.push(callParams[name] || "");
          }
        }
      }
    }
    return {
      functionName,
      args,
      abi: activeApp?.abi!,
      rpcUrl: $rpcUrl.getState(),
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
    smartAccount: $smartAccount,
    valueInputs: $valueInputs,
  }),
  clock: sendMethod,
  filter: combine(
    $activeAppWithState,
    $smartAccount,
    (app, smartAccount) => !!app && !!smartAccount && !!app.address,
  ),
  fn: ({ activeApp, params, smartAccount, valueInputs }, functionName) => {
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

    const value = valueInputs.find((v) => v.token === "NIL")?.amount;
    const tokens: Token[] = valueInputs
      .filter((valueInput) => valueInput.token !== "NIL")
      .map((valueInput) => {
        return {
          id: getTokenAddressBySymbol(valueInput.token) as Hex,
          amount: BigInt(valueInput.amount),
        };
      });

    return {
      appName: activeApp?.name,
      functionName,
      args,
      abi: activeApp?.abi!,
      rpcUrl: $rpcUrl.getState(),
      address: activeApp?.address!,
      smartAccount: smartAccount!,
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

$errors.on(sendMethodFx.done, (state, { params: { functionName } }) => {
  return {
    ...state,
    [functionName]: null,
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

$shardIdIsValid.reset($activeAppWithState);
$shardId.on(setShardId, (_, shardId) => shardId);

$activeApp.on(deploySmartContractFx.doneData, (_, { address, app }) => {
  return {
    bytecode: app,
    address,
  };
});

$activeApp.on(importSmartContractFx.doneData, (_, { importedSmartContractAddress, app }) => {
  return {
    bytecode: app,
    address: importedSmartContractAddress,
  };
});

$valueInputs
  .on(setValueInput, (state, { index, amount, token }) => {
    const newState = [...state];
    newState[index] = { amount, token };
    return newState;
  })
  .on(addValueInput, (state, availableTokens) => {
    const usedTokens = state.map((v) => v.token);
    const availableToken = availableTokens.find((c) => !usedTokens.includes(c))!;
    return [...state, { amount: "0", token: availableToken }];
  })
  .on(removeValueInput, (state, index) => state.filter((_, i) => i !== index));

$valueInputs.reset($activeAppWithState);

$shardId.on(incrementShardId, (shardId, _) => {
  return shardId === null ? 1 : shardId + 1;
});

$shardId.on(decrementShardId, (shardId, _) => {
  return shardId !== null ? Math.max(shardId - 1, 1) : 1;
});

sample({
  clock: exportApp,
  source: $activeAppWithState,
  fn: (app) =>
    ({
      abi: app?.abi,
      name: app?.name,
      sourcecode: app?.sourcecode,
      bytecode: app?.bytecode,
    }) as App,
  filter: $activeAppWithState.map((app) => !!app),
  target: exportAppFx,
});

exportAppFx.use(async (app) => {
  const JSZip = await import("jszip");
  const zip = new JSZip.default();

  zip.file(`${app.name}.bin`, app.bytecode);
  zip.file(".abi.json", JSON.stringify(app.abi, null, 2));
  zip.file(`${app.name}.sol`, app.sourcecode);

  await zip.generateAsync({ type: "blob" }).then((content) => {
    const link = document.createElement("a");
    link.href = URL.createObjectURL(content);
    link.download = `${app.name}.zip`;
    link.click();

    URL.revokeObjectURL(link.href);
  });
});

$activeComponent.on(setActiveComponent, (_, payload) => payload);

sample({
  clock: deploySmartContractFx.doneData,
  target: registerContractInCometaFx,
  source: combine({
    app: $activeAppWithState,
    cometaService: $cometaService,
    solidityVersion: $solidityVersion,
  }),
  filter: combine(
    $activeAppWithState,
    $cometaService,
    (app, cometaService) => !!app && !!cometaService,
  ),
  fn: ({ app, cometaService, solidityVersion }, { address, name }) => {
    return {
      app: app as App,
      name: name,
      address: address as Hex,
      cometaService: cometaService as CometaService,
      solidityVersion,
    };
  },
});

sample({
  source: combine({
    shardId: $shardId,
    shardsAmount: $shardsAmount,
  }),
  clock: merge([incrementShardId, decrementShardId, setShardId, triggerShardIdValidation]),
  target: $shardIdIsValid,
  fn: ({ shardId, shardsAmount }) => {
    if (shardId === null) {
      return true;
    }

    if (shardsAmount === -1) {
      return true;
    }

    return shardId <= shardsAmount && shardId >= 1;
  },
});

sample({
  clock: setRandomShardId,
  source: $shardsAmount,
  filter: $shardId.map((shardId) => shardId === null),
  target: setShardId,
  fn: (shardsAmount) => {
    if (shardsAmount === -1) {
      return 1;
    }

    const randomShardId = Math.floor(Math.random() * shardsAmount) + 1;

    return randomShardId;
  },
});

sample({
  source: $activeAppWithState,
  filter: $activeAppWithState.map((app) => !!app && !app.address),
  target: setRandomShardId,
});

sample({
  source: $activeAppWithState,
  filter: $activeAppWithState.map((app) => !!app && !app.address),
  target: triggerShardIdValidation,
});

sample({
  clock: getShardsAmountFx.doneData,
  target: $shardsAmount,
});

sample({
  clock: merge([loadedPlaygroundPage, $rpcUrl.updates]),
  target: getShardsAmountFx,
  source: $rpcUrl,
});
