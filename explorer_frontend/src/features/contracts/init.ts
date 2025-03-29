import type { CometaClient, Hex, SmartAccountV1, Token } from "@nilfoundation/niljs";
import type { Abi, AbiFunction } from "abitype";
import { combine, merge, sample } from "effector";
import { persist } from "effector-storage/local";
import { debug } from "patronum";
import { $rpcUrl, $smartAccount } from "../account-connector/model";
import {
  $solidityVersion,
  compileCodeFx,
  loadedPlaygroundPage,
  loadedTutorialPage,
} from "../code/model";
import type { App } from "../code/types";
import { $cometaClient } from "../cometa/model";
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
import {
  $callParamsValidationErrors,
  setCallParamsValidationErrors,
  validateCallParamsFx,
} from "./models/callParamsValidation";
import { exportApp, exportAppFx } from "./models/exportApp";
import { $shardsAmount, getShardsAmountFx } from "./models/shardsAmount";

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
  clock: validateCallParamsFx.doneData,
  filter: (source, doneData) => {
    const { activeApp } = source;
    const isAppValid = !!activeApp;

    const isCallMethodCalled = doneData.eventType === "callMethod";

    return isAppValid && isCallMethodCalled;
  },
  fn: ({ activeApp, params }, { functionName }) => {
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

        abiFunction.inputs.forEach((input, index) => {
          const name = input.name || index.toString();
          const paramValue = callParams[name].value;

          if (Array.isArray(paramValue)) {
            args.push(paramValue.map((v) => v.value));
          } else {
            if (input.name === "") {
              for (const key in callParams) {
                args.push(callParams[key].value || "");
              }
            } else {
              args.push(paramValue || "");
            }
          }
        });
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

$callResult.reset($activeAppWithState);
$callResult.reset(callMethod);

sample({
  source: combine({
    activeApp: $activeAppWithState,
    params: $callParams,
    smartAccount: $smartAccount,
    valueInputs: $valueInputs,
  }),
  clock: validateCallParamsFx.doneData,
  filter: (source, doneData) => {
    const { activeApp, smartAccount } = source;
    const isAppAndAccountValid = !!activeApp && !!smartAccount && !!activeApp.address;

    const isSendMethodCalled = doneData.eventType === "sendMethod";

    return isAppAndAccountValid && isSendMethodCalled;
  },
  fn: ({ activeApp, params, smartAccount, valueInputs }, { functionName }) => {
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
        abiFunction.inputs.forEach((input, index) => {
          const name = input.name || index.toString();
          const paramValue = callParams[name].value;

          if (Array.isArray(paramValue)) {
            args.push(paramValue.map((v) => v.value));
          } else {
            if (input.type === "bool") {
              args.push(paramValue === "true");
            } else {
              args.push(paramValue || "");
            }
          }
        });
      }
    }

    const functionValueInputs = valueInputs
      .filter((v) => v.functionName === functionName)
      .flatMap((v) => v.values);

    const value = functionValueInputs.find((v) => v.token === "NIL")?.amount;

    const tokens: Token[] = functionValueInputs
      .filter((valueInput) => valueInput.token !== "NIL" && valueInput.amount !== "0")
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
$txHashes.reset(sendMethod);

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

$callParams.on(setParams, (state, { functionName, paramName, value, type }) => {
  const params = state[functionName] ? { ...state[functionName] } : {};
  params[paramName] = {
    value,
    type,
  };

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

$valueInputs.on(setValueInput, (state, { index, amount, token, functionName }) => {
  const newState = [...state];

  const functionValuesIndex = newState.findIndex((v) => v.functionName === functionName);

  newState[functionValuesIndex].values[index] = {
    amount,
    token,
  };

  return newState;
});

$valueInputs.on(addValueInput, (state, { availableTokens, functionName }) => {
  const usedTokens = state
    .filter((v) => v.functionName === functionName)
    .flatMap((v) => v.values)
    .map((v) => v.token);

  const availableToken = availableTokens.find((c) => !usedTokens.includes(c));

  if (!availableToken) {
    return state;
  }

  const newState = [...state];
  const functionValuesIndex = newState.findIndex((v) => v.functionName === functionName);

  if (functionValuesIndex === -1) {
    newState.push({
      functionName,
      values: [],
    });

    return newState;
  }

  newState[functionValuesIndex].values.push({
    amount: "0",
    token: availableToken,
  });

  return newState;
});

$valueInputs.on(removeValueInput, (state, { functionName, index }) => {
  const newState = [...state];
  const functionValuesIndex = newState.findIndex((v) => v.functionName === functionName);

  newState[functionValuesIndex].values = newState[functionValuesIndex].values.filter(
    (_, i) => i !== index,
  );

  return newState;
});

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
    cometaClient: $cometaClient,
    solidityVersion: $solidityVersion,
  }),
  filter: combine(
    $activeAppWithState,
    $cometaClient,
    (app, cometaClient) => !!app && !!cometaClient,
  ),
  fn: ({ app, cometaClient, solidityVersion }, { address, name }) => {
    return {
      app: app as App,
      name: name,
      address: address as Hex,
      cometaClient: cometaClient as CometaClient,
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
  clock: merge([loadedPlaygroundPage, loadedTutorialPage, $rpcUrl.updates]),
  target: getShardsAmountFx,
  source: $rpcUrl,
});

$callParamsValidationErrors.reset($activeAppWithState);
$callParamsValidationErrors.reset(sendMethodFx.doneData);
$callParamsValidationErrors.reset(callFx.failData);
$callParamsValidationErrors.on(setCallParamsValidationErrors, (_, errors) => errors);

sample({
  clock: sendMethod,
  source: combine({
    callParams: $callParams,
    appAbi: $activeAppWithState.map((app) => app?.abi),
  }),
  target: validateCallParamsFx,
  filter: $activeAppWithState.map((app) => !!app?.abi),
  fn: ({ callParams, appAbi }, functionName) => {
    return {
      functionName,
      callParams,
      eventType: "sendMethod" as const,
      appAbi: appAbi as Abi,
    };
  },
});

sample({
  clock: callMethod,
  source: combine({
    callParams: $callParams,
    appAbi: $activeAppWithState.map((app) => app?.abi),
  }),
  target: validateCallParamsFx,
  filter: $activeAppWithState.map((app) => !!app?.abi),
  fn: ({ callParams, appAbi }, functionName) => {
    return {
      functionName,
      callParams,
      eventType: "callMethod" as const,
      appAbi: appAbi as Abi,
    };
  },
});

sample({
  clock: setParams,
  source: $callParamsValidationErrors,
  target: $callParamsValidationErrors,
  fn: (errors, setParamsPayload) => {
    const newErrors = { ...errors };
    const functionErrors = newErrors[setParamsPayload.functionName];

    if (!functionErrors) {
      return newErrors;
    }

    functionErrors[setParamsPayload.paramName] = null;

    newErrors[setParamsPayload.functionName] = functionErrors;

    return newErrors;
  },
});

sample({
  source: $activeAppWithState,
  target: $valueInputs,
  filter: combine($activeAppWithState, $valueInputs, (app, valueInputs) => {
    if (app === null) {
      return false;
    }

    return valueInputs.length === 0;
  }),
  fn: (app) => {
    const payableFunctionNames = app?.abi
      .filter((abi) => abi.type === "function" && abi.stateMutability === "payable")
      .map((abi) => (abi as AbiFunction).name);

    if (!payableFunctionNames || payableFunctionNames.length === 0) {
      return [];
    }

    const defaultValuesToSet = payableFunctionNames.map((functionName) => {
      return {
        functionName,
        values: [
          {
            token: "NIL",
            amount: "0",
          },
        ],
      };
    });

    return defaultValuesToSet;
  },
});
