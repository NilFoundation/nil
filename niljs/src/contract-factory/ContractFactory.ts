import type {
  Abi,
  AbiEvent,
  AbiFunction,
  AbiParameter,
  AbiParameterToPrimitiveType,
  AbiParametersToPrimitiveTypes,
  AbiStateMutability,
  Address,
  ExtractAbiError,
  ExtractAbiErrorNames,
  ExtractAbiEvent,
  ExtractAbiEventNames,
  ExtractAbiFunction,
  ExtractAbiFunctionNames,
  ResolvedRegister,
} from "abitype";
import type { AbiConstructor, AbiError, AbiFallback, AbiReceive } from "abitype";
import { AbiFunction as AbiFunctionZod, Abi as AbiZod } from "abitype/zod";
import type { Prettify } from "ts-essentials";
import type { PublicClient } from "../clients/PublicClient.js";
import type { ISigner } from "../signers/types/ISigner.js";
import type {
  SendTransactionParams,
  SmartAccountInterface,
} from "../smart-accounts/SmartAccountInterface.js";
import type { CallArgs } from "../types/CallArgs.js";
import type { Hex } from "../types/Hex.js";
import type {
  ArrayToObject,
  ExternalContractsMethod,
  Filter,
  IsNarrowable,
  IsUnion,
  MaybeRequired,
  ReadContractsMethod,
  UnionToTuple,
  WriteContractsMethod,
} from "../types/utils.js";
import {
  contractInteraction,
  writeContract,
  writeExternalContract,
} from "./contractInteraction.js";
import type { WriteOptions } from "./contractInteraction.js";

export type LogTopic = Hex | Hex[] | null;

export type ContractFunctionName<
  abi extends Abi | readonly unknown[] = Abi,
  mutability extends AbiStateMutability = AbiStateMutability,
> = ExtractAbiFunctionNames<
  abi extends Abi ? abi : Abi,
  mutability
> extends infer functionName extends string
  ? [functionName] extends [never]
    ? string
    : functionName
  : string;

export type ContractErrorName<abi extends Abi | readonly unknown[] = Abi> = ExtractAbiErrorNames<
  abi extends Abi ? abi : Abi
> extends infer errorName extends string
  ? [errorName] extends [never]
    ? string
    : errorName
  : string;

export type ContractEventName<abi extends Abi | readonly unknown[] = Abi> = ExtractAbiEventNames<
  abi extends Abi ? abi : Abi
> extends infer eventName extends string
  ? [eventName] extends [never]
    ? string
    : eventName
  : string;

export type ContractFunctionArgs<
  abi extends Abi | readonly unknown[] = Abi,
  mutability extends AbiStateMutability = AbiStateMutability,
  functionName extends ContractFunctionName<abi, mutability> = ContractFunctionName<
    abi,
    mutability
  >,
> = AbiParametersToPrimitiveTypes<
  ExtractAbiFunction<abi extends Abi ? abi : Abi, functionName, mutability>["inputs"],
  "inputs"
> extends infer args
  ? [args] extends [never]
    ? readonly unknown[]
    : args
  : readonly unknown[];

export type ContractConstructorArgs<abi extends Abi | readonly unknown[] = Abi> =
  AbiParametersToPrimitiveTypes<
    Extract<(abi extends Abi ? abi : Abi)[number], { type: "constructor" }>["inputs"],
    "inputs"
  > extends infer args
    ? [args] extends [never]
      ? readonly unknown[]
      : args
    : readonly unknown[];

export type ContractErrorArgs<
  abi extends Abi | readonly unknown[] = Abi,
  errorName extends ContractErrorName<abi> = ContractErrorName<abi>,
> = AbiParametersToPrimitiveTypes<
  ExtractAbiError<abi extends Abi ? abi : Abi, errorName>["inputs"],
  "inputs"
> extends infer args
  ? [args] extends [never]
    ? readonly unknown[]
    : args
  : readonly unknown[];

export type ContractEventArgs<
  abi extends Abi | readonly unknown[] = Abi,
  eventName extends ContractEventName<abi> = ContractEventName<abi>,
> = AbiEventParametersToPrimitiveTypes<
  ExtractAbiEvent<abi extends Abi ? abi : Abi, eventName>["inputs"]
> extends infer args
  ? [args] extends [never]
    ? readonly unknown[] | Record<string, unknown>
    : args
  : readonly unknown[] | Record<string, unknown>;

export type ContractEventArgsFromTopics<
  abi extends Abi | readonly unknown[] = Abi,
  eventName extends ContractEventName<abi> = ContractEventName<abi>,
  strict extends boolean = true,
> = AbiEventParametersToPrimitiveTypes<
  ExtractAbiEvent<abi extends Abi ? abi : Abi, eventName>["inputs"],
  { EnableUnion: false; IndexedOnly: false; Required: strict }
> extends infer args
  ? [args] extends [never]
    ? readonly unknown[] | Record<string, unknown>
    : args
  : readonly unknown[] | Record<string, unknown>;

export type Widen<T> =
  | ([unknown] extends [T] ? unknown : never)
  // biome-ignore lint/complexity/noBannedTypes: just to compile
  | (T extends Function ? T : never)
  | (T extends ResolvedRegister["BigIntType"] ? bigint : never)
  | (T extends boolean ? boolean : never)
  | (T extends ResolvedRegister["IntType"] ? number : never)
  | (T extends string
      ? T extends ResolvedRegister["AddressType"]
        ? ResolvedRegister["AddressType"]
        : T extends ResolvedRegister["BytesType"]["inputs"]
          ? ResolvedRegister["BytesType"]
          : string
      : never)
  | (T extends readonly [] ? readonly [] : never)
  | (T extends Record<string, unknown> ? { [K in keyof T]: Widen<T[K]> } : never)
  | (T extends { length: number }
      ? {
          [K in keyof T]: Widen<T[K]>;
        } extends infer Val extends readonly unknown[]
        ? readonly [...Val]
        : never
      : never);

// biome-ignore lint/suspicious/noExplicitAny: it could be any options here
export type UnionWiden<T> = T extends any ? Widen<T> : never;

export type ExtractAbiFunctionForArgs<
  abi extends Abi,
  mutability extends AbiStateMutability,
  functionName extends ContractFunctionName<abi, mutability>,
  args extends ContractFunctionArgs<abi, mutability, functionName>,
> = ExtractAbiFunction<abi, functionName, mutability> extends infer abiFunction extends AbiFunction
  ? IsUnion<abiFunction> extends true // narrow overloads using `args` by converting to tuple and filtering out overloads that don't match
    ? UnionToTuple<abiFunction> extends infer abiFunctions extends readonly AbiFunction[]
      ? // convert back to union (removes `never` tuple entries)
        { [k in keyof abiFunctions]: CheckArgs<abiFunctions[k], args> }[number]
      : never
    : abiFunction
  : never;
type CheckArgs<
  abiFunction extends AbiFunction,
  args,
  ///
  targetArgs extends AbiParametersToPrimitiveTypes<
    abiFunction["inputs"],
    "inputs"
  > = AbiParametersToPrimitiveTypes<abiFunction["inputs"], "inputs">,
> = (readonly [] extends args ? readonly [] : args) extends targetArgs // fallback to `readonly []` if `args` has no value (e.g. `args` property not provided)
  ? abiFunction
  : never;

export type ContractFunctionParameters<
  abi extends Abi | readonly unknown[] = Abi,
  mutability extends AbiStateMutability = AbiStateMutability,
  functionName extends ContractFunctionName<abi, mutability> = ContractFunctionName<
    abi,
    mutability
  >,
  args extends ContractFunctionArgs<abi, mutability, functionName> = ContractFunctionArgs<
    abi,
    mutability,
    functionName
  >,
  deployless extends boolean = false,
  ///
  allFunctionNames = ContractFunctionName<abi, mutability>,
  allArgs = ContractFunctionArgs<abi, mutability, functionName>,
  // when `args` is inferred to `readonly []` ("inputs": []) or `never` (`abi` declared as `Abi` or not inferrable), allow `args` to be optional.
  // important that both branches return same structural type
> = {
  abi: abi;
  functionName:
    | allFunctionNames // show all options
    | (functionName extends allFunctionNames ? functionName : never); // infer value
  args?: (abi extends Abi ? UnionWiden<args> : never) | allArgs | undefined;
  // biome-ignore lint/complexity/noBannedTypes: just to compile
} & (readonly [] extends allArgs ? {} : { args: Widen<args> }) &
  (deployless extends true ? { address?: undefined; code: Hex } : { address: Address });

export type ContractFunctionReturnType<
  abi extends Abi | readonly unknown[] = Abi,
  mutability extends AbiStateMutability = AbiStateMutability,
  functionName extends ContractFunctionName<abi, mutability> = ContractFunctionName<
    abi,
    mutability
  >,
  args extends ContractFunctionArgs<abi, mutability, functionName> = ContractFunctionArgs<
    abi,
    mutability,
    functionName
  >,
> = abi extends Abi
  ? Abi extends abi
    ? unknown
    : AbiParametersToPrimitiveTypes<
          ExtractAbiFunctionForArgs<abi, mutability, functionName, args>["outputs"]
        > extends infer types
      ? types extends readonly []
        ? // biome-ignore lint/suspicious/noConfusingVoidType: can be void
          void
        : types extends readonly [infer T]
          ? T
          : types
      : never
  : unknown;

export type AbiItem = Abi[number];

export type ExtractAbiItemNames<abi extends Abi> = Extract<abi[number], { name: string }>["name"];

export type ExtractAbiItem<abi extends Abi, name extends ExtractAbiItemNames<abi>> = Extract<
  abi[number],
  { name: name }
>;

export type AbiItemName<abi extends Abi | readonly unknown[] = Abi> = abi extends Abi
  ? ExtractAbiItemNames<abi>
  : string;

export type AbiItemArgs<
  abi extends Abi | readonly unknown[] = Abi,
  name extends AbiItemName<abi> = AbiItemName<abi>,
> = AbiParametersToPrimitiveTypes<
  ExtractAbiItem<abi extends Abi ? abi : Abi, name>["inputs"],
  "inputs"
> extends infer args
  ? [args] extends [never]
    ? readonly unknown[]
    : args
  : readonly unknown[];

export type ExtractAbiItemForArgs<
  abi extends Abi,
  name extends AbiItemName<abi>,
  args extends AbiItemArgs<abi, name>,
> = ExtractAbiItem<abi, name> extends infer abiItem extends AbiItem & {
  inputs: readonly AbiParameter[];
}
  ? IsUnion<abiItem> extends true // narrow overloads using `args` by converting to tuple and filtering out overloads that don't match
    ? UnionToTuple<abiItem> extends infer abiItems extends readonly (AbiItem & {
        inputs: readonly AbiParameter[];
      })[]
      ? {
          [k in keyof abiItems]: (
            readonly [] extends args
              ? readonly [] // fallback to `readonly []` if `args` has no value (e.g. `args` property not provided)
              : args
          ) extends AbiParametersToPrimitiveTypes<abiItems[k]["inputs"], "inputs">
            ? abiItems[k]
            : never;
        }[number] // convert back to union (removes `never` tuple entries: `['foo', never, 'bar'][number]` => `'foo' | 'bar'`)
      : never
    : abiItem
  : never;

export type EventDefinition = `${string}(${string})`;

export type GetValue<
  abi extends Abi | readonly unknown[],
  functionName extends string,
  valueType = unknown,
  abiFunction extends AbiFunction = abi extends Abi
    ? ExtractAbiFunction<abi, functionName>
    : AbiFunction,
  Narrowable extends boolean = IsNarrowable<abi, Abi>,
> = Narrowable extends true
  ? abiFunction["stateMutability"] extends "payable"
    ? { value?: NoInfer<valueType> | undefined }
    : abiFunction["payable"] extends true
      ? { value?: NoInfer<valueType> | undefined }
      : { value?: undefined }
  : { value?: NoInfer<valueType> | undefined };

//////////////////////////////////////////////////////////////////////////////////////////////////

export type MaybeAbiEventName<abiEvent extends AbiEvent | undefined> = abiEvent extends AbiEvent
  ? abiEvent["name"]
  : undefined;

export type MaybeExtractEventArgsFromAbi<
  abi extends Abi | readonly unknown[] | undefined,
  eventName extends string | undefined,
> = abi extends Abi | readonly unknown[]
  ? eventName extends string
    ? GetEventArgs<abi, eventName>
    : undefined
  : undefined;

//////////////////////////////////////////////////////////////////////
// ABI item args

export type GetEventArgs<
  abi extends Abi | readonly unknown[],
  eventName extends string,
  config extends EventParameterOptions = DefaultEventParameterOptions,
  abiEvent extends AbiEvent & { type: "event" } = abi extends Abi
    ? ExtractAbiEvent<abi, eventName>
    : AbiEvent & { type: "event" },
  args = AbiEventParametersToPrimitiveTypes<abiEvent["inputs"], config>,
  FailedToParseArgs =
    | ([args] extends [never] ? true : false)
    | (readonly unknown[] extends args ? true : false),
> = true extends FailedToParseArgs ? readonly unknown[] | Record<string, unknown> : args;

//////////////////////////////////////////////////////////////////////
// ABI event types

type EventParameterOptions = {
  EnableUnion?: boolean;
  IndexedOnly?: boolean;
  Required?: boolean;
};
type DefaultEventParameterOptions = {
  EnableUnion: true;
  IndexedOnly: true;
  Required: false;
};

export type AbiEventParametersToPrimitiveTypes<
  abiParameters extends readonly AbiParameter[],
  //
  options extends EventParameterOptions = DefaultEventParameterOptions,
  // Remove non-indexed parameters based on `Options['IndexedOnly']`
> = abiParameters extends readonly []
  ? readonly []
  : Filter<
        abiParameters,
        options["IndexedOnly"] extends true ? { indexed: true } : object
      > extends infer Filtered extends readonly AbiParameter[]
    ? HasUnnamedAbiParameter<Filtered> extends true
      ? // Has unnamed tuple parameters so return as array
          | readonly [
              ...{
                [K in keyof Filtered]: AbiEventParameterToPrimitiveType<Filtered[K], options>;
              },
            ]
          // Distribute over tuple to represent optional parameters
          | (options["Required"] extends true
              ? never
              : // Distribute over tuple to represent optional parameters
                Filtered extends readonly [
                    ...infer Head extends readonly AbiParameter[],
                    infer param,
                  ]
                ? AbiEventParametersToPrimitiveTypes<
                    readonly [...{ [K in keyof Head]: Omit<Head[K], "name"> }],
                    options
                  >
                : never)
      : // All tuple parameters are named so return as object
        {
            [Parameter in Filtered[number] as Parameter extends {
              name: infer Name extends string;
            }
              ? Name
              : never]?: AbiEventParameterToPrimitiveType<Parameter, options> | undefined;
          } extends infer Mapped
        ? Prettify<
            MaybeRequired<Mapped, options["Required"] extends boolean ? options["Required"] : false>
          >
        : never
    : never;

export type UnionEvaluate<T> = T extends object ? Prettify<T> : T;

// TODO: Speed up by returning immediately as soon as named parameter is found.
type HasUnnamedAbiParameter<abiParameters extends readonly AbiParameter[]> =
  abiParameters extends readonly [
    infer Head extends AbiParameter,
    ...infer Tail extends readonly AbiParameter[],
  ]
    ? Head extends { name: string }
      ? Head["name"] extends ""
        ? true
        : HasUnnamedAbiParameter<Tail>
      : true
    : false;
export type ReadContractParameters<
  abi extends Abi | readonly unknown[] = Abi,
  functionName extends ContractFunctionName<abi, "pure" | "view"> = ContractFunctionName<
    abi,
    "pure" | "view"
  >,
  args extends ContractFunctionArgs<abi, "pure" | "view", functionName> = ContractFunctionArgs<
    abi,
    "pure" | "view",
    functionName
  >,
> = UnionEvaluate<Pick<CallArgs, "to">> &
  ContractFunctionParameters<abi, "pure" | "view", functionName, args, false>;

export type WriteContractParameters<
  abi extends Abi | readonly unknown[] = Abi,
  functionName extends ContractFunctionName<abi, "payable" | "nonpayable"> = ContractFunctionName<
    abi,
    "payable" | "nonpayable"
  >,
  args extends ContractFunctionArgs<
    abi,
    "payable" | "nonpayable",
    functionName
  > = ContractFunctionArgs<abi, "payable" | "nonpayable", functionName>,
> = UnionEvaluate<
  Pick<
    SendTransactionParams,
    "to" | "refundTo" | "bounceTo" | "tokens" | "value" | "seqno" | "feeCredit"
  >
> &
  ContractFunctionParameters<abi, "nonpayable" | "payable", functionName, args, false>;

/**
 * @internal
 */
export type LogTopicType<primitiveType = Hex, topic extends LogTopic = LogTopic> = topic extends Hex
  ? primitiveType
  : topic extends Hex[]
    ? primitiveType[]
    : topic extends null
      ? null
      : never;

/**
 * @internal
 */
export type AbiEventParameterToPrimitiveType<
  abiParameter extends AbiParameter,
  //
  options extends EventParameterOptions = DefaultEventParameterOptions,
  type = AbiParameterToPrimitiveType<abiParameter>,
> = options["EnableUnion"] extends true ? LogTopicType<type> : type;

type HashedEventTypes = "bytes" | "string" | "tuple" | `${string}[${string}]`;

/**
 * @internal
 */
export type AbiEventTopicToPrimitiveType<
  abiParameter extends AbiParameter,
  topic extends LogTopic,
  primitiveType = abiParameter["type"] extends HashedEventTypes
    ? topic
    : AbiParameterToPrimitiveType<abiParameter>,
> = LogTopicType<primitiveType, topic>;

export type GetContractParams<
  client extends PublicClient,
  abi extends ReadonlyArray<
    AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
  >,
> = {
  abi: abi;
  client: client;
  address: Address;
};
export type CommonReadContractMethods = Record<string, (args: unknown[]) => unknown[] | unknown>;
export type CommonWriteContractMethods = Record<
  string,
  (args: unknown[], writeOptions?: WriteOptions) => Promise<Hex>
>;

export function getContract<
  A extends Abi | unknown[],
  C extends PublicClient,
  W extends SmartAccountInterface,
>(params: {
  abi: A;
  client: C;
  smartAccount: W;
  address: Address;
}): {
  read: A extends Abi ? ReadContractsMethod<A, "view" | "pure"> : CommonReadContractMethods;
  write: A extends Abi
    ? WriteContractsMethod<A, "payable" | "nonpayable">
    : CommonWriteContractMethods;
};

export function getContract<
  A extends Abi | unknown[],
  C extends PublicClient,
  W extends SmartAccountInterface,
  K extends string,
>(params: {
  abi: A;
  client: C;
  smartAccount: W;
  address: Address;
  externalInterface?: { signer: ISigner; methods: K[] };
}): {
  read: A extends Abi ? ReadContractsMethod<A, "view" | "pure"> : CommonReadContractMethods;
  write: A extends Abi
    ? WriteContractsMethod<A, "payable" | "nonpayable">
    : CommonWriteContractMethods;
  external: A extends Abi
    ? ExternalContractsMethod<A, "payable" | "nonpayable", K[]>
    : CommonWriteContractMethods;
};

export function getContract<
  A extends Abi | unknown[],
  C extends PublicClient,
  W extends SmartAccountInterface,
  K extends string,
>(params: {
  abi: A;
  client: C;
  smartAccount: W;
  address: Address;
  externalInterface?: { methods: K[] };
}): {
  read: A extends Abi ? ReadContractsMethod<A, "view" | "pure"> : CommonReadContractMethods;
  write: A extends Abi
    ? WriteContractsMethod<A, "payable" | "nonpayable">
    : CommonWriteContractMethods;
};

export function getContract<A extends Abi | unknown[], C extends PublicClient>(params: {
  abi: A;
  client: C;
  address: Address;
}): {
  read: A extends Abi ? ReadContractsMethod<A, "view" | "pure"> : CommonReadContractMethods;
};

export function getContract<
  A extends Abi | unknown[],
  C extends PublicClient,
  W extends SmartAccountInterface,
  K extends string,
>({
  abi,
  client,
  smartAccount,
  address,
  externalInterface,
}: {
  abi: A;
  client: C;
  smartAccount?: W;
  address: Address;
  externalInterface?: { signer?: ISigner; methods: K[] };
}) {
  const parseResult = AbiZod.safeParse(abi);
  if (parseResult.error) {
    throw parseResult.error;
  }

  const resolvedAbi = parseResult.data as A extends Abi ? A : Abi;
  const filteredAbiOnlyFunction = resolvedAbi.filter(
    (item) => item.type === "function",
  ) as A extends Abi ? Filter<A, AbiFunction> : AbiFunction[];

  const readMethods = {} as A extends Abi
    ? Partial<ArrayToObject<typeof filteredAbiOnlyFunction, "view" | "pure">>
    : Record<string, AbiFunction>;

  for (let i = 0; i < filteredAbiOnlyFunction.length; i++) {
    const t: (typeof filteredAbiOnlyFunction)[typeof i] = filteredAbiOnlyFunction[i];
    const parsedAbi = AbiFunctionZod.safeParse(t);
    if (parsedAbi.success) {
      const b = parsedAbi.data as A extends Abi
        ? (typeof filteredAbiOnlyFunction)[typeof i]
        : AbiFunction;
      if (b.type === "function" && (b.stateMutability === "view" || b.stateMutability === "pure")) {
        // @ts-ignore
        readMethods[b.name] = b;
      }
    }
  }

  const completedReadMethods = readMethods as A extends Abi
    ? ArrayToObject<typeof filteredAbiOnlyFunction, "view" | "pure">
    : Record<string, AbiFunction>;

  const readProxy = new Proxy(completedReadMethods, {
    // biome-ignore lint/suspicious/noExplicitAny: it could be any options here
    get(target, p): any {
      if (p in target) {
        const item = target[p as keyof typeof target];
        return async (args: unknown[]) => {
          // @ts-ignore
          const result = await contractInteraction(client, {
            to: address,
            abi,
            // @ts-ignore
            functionName: item.name,
            args: args,
          });
          return result;
        };
      }
      throw new Error("No read method");
    },
  });

  if (!smartAccount) {
    return {
      read: readProxy as unknown,
    } as {
      read: A extends Abi ? ReadContractsMethod<A, "view" | "pure"> : CommonReadContractMethods;
    };
  }

  const writeMethods = {} as A extends Abi
    ? Partial<ArrayToObject<A, "nonpayable" | "payable">>
    : Record<string, AbiFunction>;
  for (let i = 0; i < abi.length; i++) {
    const t: A[typeof i] = abi[i];
    const parsedAbi = AbiFunctionZod.safeParse(t);
    if (parsedAbi.success) {
      const b = parsedAbi.data;
      if (
        b.type === "function" &&
        (b.stateMutability === "payable" || b.stateMutability === "nonpayable")
      ) {
        // @ts-ignore
        writeMethods[b.name] = b;
      }
    }
  }

  const mySmartAccount = smartAccount as SmartAccountInterface;

  // @ts-ignore
  const completeWriteMethods = writeMethods as ArrayToObject<A, "nonpayable" | "payable">;

  const writeProxy = new Proxy(completeWriteMethods, {
    get: (target, property) => {
      if (typeof property === "symbol") {
        throw new Error("No write method");
      }
      if (property in target) {
        return async (args: unknown[], options: WriteOptions) => {
          return writeContract({
            smartAccount: mySmartAccount,
            abi,
            // @ts-ignore
            functionName: property,
            // @ts-ignore
            args,
            options: {
              ...options,
              to: address,
            },
          });
        };
      }
      throw new Error(`No write method with name ${property}`);
    },
  });

  if (!externalInterface || !externalInterface.signer) {
    return {
      read: readProxy as unknown,
      write: writeProxy as unknown,
    } as {
      read: A extends ReadonlyArray<
        AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
      >
        ? ReadContractsMethod<A, "view" | "pure">
        : CommonReadContractMethods;
      write: A extends ReadonlyArray<
        AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
      >
        ? WriteContractsMethod<A, "payable" | "nonpayable">
        : CommonWriteContractMethods;
    };
  }
  const signer = externalInterface.signer;

  const writeExternalMethods = {} as A extends Abi
    ? Partial<ArrayToObject<A, "nonpayable" | "payable">>
    : Record<string, AbiFunction>;
  for (let i = 0; i < abi.length; i++) {
    const t: A[typeof i] = abi[i];
    const parsedAbi = AbiFunctionZod.safeParse(t);
    if (parsedAbi.success) {
      const b = parsedAbi.data;
      if (
        b.type === "function" &&
        (b.stateMutability === "payable" || b.stateMutability === "nonpayable") &&
        externalInterface.methods.includes(b.name as K)
      ) {
        // @ts-ignore
        writeExternalMethods[b.name] = b;
      }
    }
  }
  const completeExternalMethods = writeExternalMethods as ArrayToObject<
    // @ts-ignore
    A,
    "nonpayable" | "payable"
  >;

  const writeExternalProxy = new Proxy(completeExternalMethods, {
    get: (target, property) => {
      if (typeof property === "symbol") {
        throw new Error("No write method");
      }
      if (property in target) {
        return async (args: unknown[], options: WriteOptions) => {
          return writeExternalContract({
            client,
            signer,
            abi,
            // @ts-ignore
            functionName: property,
            // @ts-ignore
            args,
            options: {
              ...options,
              to: address,
            },
          });
        };
      }
      throw new Error(`No write method with name ${property}`);
    },
  });

  return {
    read: readProxy as unknown,
    write: writeProxy as unknown,
    external: writeExternalProxy as unknown,
  } as {
    read: A extends ReadonlyArray<
      AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
    >
      ? ReadContractsMethod<A, "view" | "pure">
      : CommonReadContractMethods;
    write: A extends ReadonlyArray<
      AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
    >
      ? WriteContractsMethod<A, "payable" | "nonpayable">
      : CommonWriteContractMethods;
    external: A extends ReadonlyArray<
      AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
    >
      ? ExternalContractsMethod<A, "payable" | "nonpayable", K[]>
      : CommonWriteContractMethods;
  };
}
