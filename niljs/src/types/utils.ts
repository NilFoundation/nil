import type {
  Abi,
  AbiConstructor,
  AbiError,
  AbiEvent,
  AbiFallback,
  AbiFunction,
  AbiParametersToPrimitiveTypes,
  AbiReceive,
  AbiStateMutability,
} from "abitype";
import type { DeepRequired, IsNever, UnionToIntersection } from "ts-essentials";
import type {
  ContractFunctionArgs,
  ContractFunctionName,
  ContractFunctionReturnType,
} from "../contract-factory/ContractFactory.js";
import type { WriteOptions } from "../contract-factory/contractInteraction.js";
import type { Hex } from "./Hex.js";

export type IsUnion<
  union,
  ///
  union2 = union,
> = union extends union2 ? ([union2] extends [union] ? false : true) : never;

export type UnionToTuple<
  union,
  ///
  last = LastInUnion<union>,
> = [union] extends [never] ? [] : [...UnionToTuple<Exclude<union, last>>, last];
type LastInUnion<U> = UnionToIntersection<U extends unknown ? (x: U) => 0 : never> extends (
  x: infer l,
) => 0
  ? l
  : never;

/**
 * @description Checks if {@link T} can be narrowed further than {@link U}
 * @param T - Type to check
 * @param U - Type to against
 * @example
 * type Result = IsNarrowable<'foo', string>
 * //   ^? true
 */
export type IsNarrowable<T, U> = IsNever<
  (T extends U ? true : false) & (U extends T ? false : true)
> extends true
  ? false
  : true;

/**
 * @description Makes attributes on the type T required if required is true.
 *
 * @example
 * MaybeRequired<{ a: string, b?: number }, true>
 * => { a: string, b: number }
 *
 * MaybeRequired<{ a: string, b?: number }, false>
 * => { a: string, b?: number }
 */
export type MaybeRequired<T, required extends boolean> = required extends true
  ? DeepRequired<T>
  : T;

/**
 * Filters out all members of {@link T} that are not {@link P}
 *
 * @param T - Items to filter
 * @param P - Type to filter out
 * @returns Filtered items
 *
 * @example
 * type Result = Filter<['a', 'b', 'c'], 'b'>
 * //   ^? type Result = ['a', 'c']
 */
export type Filter<
  T extends readonly unknown[],
  P,
  Acc extends readonly unknown[] = [],
> = T extends readonly [infer F, ...infer Rest extends readonly unknown[]]
  ? [F] extends [P]
    ? Filter<Rest, P, [...Acc, F]>
    : Filter<Rest, P, Acc>
  : readonly [...Acc];

export type ReadContractReturnType<
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
> = ContractFunctionReturnType<abi, "pure" | "view", functionName, args>;

export type ArrayToObject<
  T extends Abi,
  S extends AbiStateMutability = AbiStateMutability,
  B extends Extract<
    T[number],
    {
      type: "function";
      stateMutability: S;
    }
  > = Extract<
    T[number],
    {
      type: "function";
      stateMutability: S;
    }
  >,
> = {
  [K in B["name"]]: B;
};

type SingleOrTuple<T extends readonly unknown[]> = T["length"] extends 1 ? T[0] : T;

type ArgsOrNone<T extends readonly unknown[]> = T["length"] extends 0 ? T | undefined : T;

export type ReadContractsMethod<
  T extends ReadonlyArray<
    AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
  >,
  S extends AbiStateMutability = "pure" | "view",
  B extends AbiFunction = Extract<
    T[number],
    {
      stateMutability: S;
      type: "function";
    }
  >,
  F extends ContractFunctionName<T, S> = ContractFunctionName<T, S>,
> = {
  [K in F]: (
    args: ContractFunctionArgs<T, S, F>,
  ) => SingleOrTuple<AbiParametersToPrimitiveTypes<B["outputs"], "outputs">>;
};

export type WriteContractsMethod<
  T extends ReadonlyArray<
    AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
  >,
  S extends AbiStateMutability = "payable" | "nonpayable",
  B extends AbiFunction = Extract<
    T[number],
    {
      stateMutability: S;
      type: "function";
    }
  >,
> = {
  [K in B["name"]]: (
    args: AbiParametersToPrimitiveTypes<B["inputs"], "inputs">,
    options?: WriteOptions,
  ) => Promise<Hex>;
};

export type ExternalContractsMethod<
  T extends ReadonlyArray<
    AbiConstructor | AbiError | AbiEvent | AbiFallback | AbiFunction | AbiReceive
  >,
  S extends AbiStateMutability = "payable" | "nonpayable",
  F extends readonly string[] = readonly string[],
  B extends AbiFunction = Extract<
    T[number],
    {
      stateMutability: S;
      type: "function";
      name: F[number];
    }
  >,
> = {
  [K in B["name"]]: (
    args: AbiParametersToPrimitiveTypes<B["inputs"], "inputs">,
    options?: WriteOptions,
  ) => Promise<Hex>;
};

const el = {
  type: "function",
  name: "foo",
  inputs: [
    {
      type: "uint256",
      name: "a",
    },
  ],
  outputs: [{ type: "uint256" }],
  stateMutability: "view",
} as const;
const dd: Extract<typeof el, { type: "function"; stateMutability: "view" }> = el;

const a = [el] as const;

const d: ArrayToObject<typeof a, "view" | "pure"> = {
  foo: a[0],
};

const b: Partial<ArrayToObject<typeof a, "view">> = {
  foo: a[0],
};

for (const t of a) {
  b[t.name] = t;
}

const c = b as ArrayToObject<typeof a, "view">;
// @ts-ignore
const t: AbiParametersToPrimitiveTypes<typeof el.inputs> = 1;
