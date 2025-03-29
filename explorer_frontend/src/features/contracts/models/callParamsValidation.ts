import { isAddress, isHexString } from "@nilfoundation/niljs";
import type { Abi, AbiFunction } from "abitype";
import { createDomain } from "effector";
import type { CallParams } from "../types";
import { isAbiParameterTuple } from "../utils";
import type { $callParams } from "./base";

const uintValidation = {
  fn: (value: string | boolean) => {
    if (!value) {
      return false;
    }

    try {
      const valueAsBigint = BigInt(value);
      return valueAsBigint >= 0;
    } catch (e) {
      return false;
    }
  },
  err: "Value should be an unsigned integer (0 to 2^256 - 1)",
} as const;

const intValidation = {
  fn: (value: string | boolean) => {
    if (!value) {
      return false;
    }

    try {
      BigInt(value);
      return true;
    } catch (e) {
      return false;
    }
  },
  err: "Value should be an integer",
} as const;

const addressValidation = {
  fn: (value: string | boolean) => {
    return !!value && typeof value === "string" && isAddress(value);
  },
  err: "Address should be a valid hex string of 42 characters length with 0x prefix",
} as const;

const bytesValidation = {
  fn: (value: string | boolean) => {
    return !!value && isHexString(value);
  },
  err: "Bytes should be a valid hex string",
} as const;

const boolValidation = {
  fn: (value: string | boolean) => {
    return typeof value === "boolean" || value === "true" || value === "false";
  },
  err: "Value should be a boolean",
} as const;

const validations = {
  address: addressValidation,
  bytes: bytesValidation,
  bool: boolValidation,
  ...Object.fromEntries(
    ["uint", "uint8", "uint16", "uint32", "uint64", "uint128", "uint256"].map((type) => [
      type,
      uintValidation,
    ]),
  ),
  ...Object.fromEntries(
    ["int", "int8", "int16", "int32", "int64", "int128", "int256"].map((type) => [
      type,
      intValidation,
    ]),
  ),
} as const;

export const codeDomain = createDomain("call-params-validation");

export const $callParamsValidationErrors = codeDomain.createStore<
  Record<string, Record<string, string | null | Record<string, string | null>>>
>({});

export const setCallParamsValidationErrors =
  codeDomain.createEvent<
    Record<string, Record<string, string | null | Record<string, string | null>>>
  >();

export const validateCallParamsFx = codeDomain.createEffect<
  {
    functionName: string;
    callParams: ReturnType<typeof $callParams.getState>;
    appAbi: Abi;
    eventType: "callMethod" | "sendMethod";
  },
  {
    functionName: string;
    eventType: "callMethod" | "sendMethod";
  }
>();

validateCallParamsFx.use(async ({ functionName, callParams, eventType, appAbi }) => {
  const newErrors: Record<string, string | null | Record<string, string | null>> = {};

  let abiFunction: AbiFunction | null = null;
  for (const abiField of appAbi) {
    if (abiField.type === "function" && abiField.name === functionName) {
      abiFunction = abiField;
      break;
    }
  }

  const allFunctionParams: CallParams[string] = {};

  if (abiFunction) {
    abiFunction.inputs.forEach((input, index) => {
      const name = input.name || index.toString();
      const value = callParams[functionName]?.[name]?.value;
      const type = input.type;

      if (isAbiParameterTuple(input)) {
        const components = input.components || [];

        const tupleValue = [] as { value: string | boolean; type: string }[];

        components.forEach((component, componentIndex) => {
          const valueByIndex = Array.isArray(value) ? value[componentIndex] : null;
          tupleValue[componentIndex] = valueByIndex ?? {
            value: "",
            type: component.type,
          };
        });

        allFunctionParams[name] = { value: tupleValue, type };
        return;
      }

      allFunctionParams[name] = { value, type };
    });
  }

  for (const [paramName, { value, type }] of Object.entries(allFunctionParams)) {
    const validation = validations[type as keyof typeof validations];

    if (Array.isArray(value)) {
      value.forEach((item, index) => {
        const itemValidation = validations[item.type as keyof typeof validations];

        if (!itemValidation) {
          return;
        }

        const isValid = itemValidation.fn(item.value);

        if (!isValid) {
          if (!newErrors[paramName]) {
            newErrors[paramName] = {};
          }

          newErrors[paramName] = {
            ...(newErrors[paramName] as Record<string, string | null>),
            ...{ [index.toString()]: itemValidation.err },
          };
        }
      });

      continue;
    }

    if (!validation) {
      continue;
    }

    const isValid = validation.fn(value);

    if (!isValid) {
      newErrors[paramName] = validation.err;
    }
  }

  const isValid = Object.keys(newErrors).length === 0;

  if (!isValid) {
    setCallParamsValidationErrors({ [functionName]: newErrors });
    throw new Error("Validation failed");
  }

  return { functionName, eventType };
});
