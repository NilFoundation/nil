import type { AbiInternalType, AbiParameter } from "abitype";

export function getResultDisplay(result: unknown): string {
  if (Array.isArray(result)) {
    return `[ ${result.map(getResultDisplay).join(", ")} ]`;
  }

  if (typeof result === "object" && result !== null) {
    const entries = Object.entries(result)
      .filter(([key]) => !/^\d+$/.test(key))
      .map(([key, value]) => `${key}: ${getResultDisplay(value)}`);

    return `{ ${entries.join(", ")} }`;
  }

  return String(result);
}

export const isAbiParameterTuple = (
  input: AbiParameter,
): input is {
  type: "tuple" | `tuple[${string}]`;
  name?: string | undefined;
  internalType?: AbiInternalType | undefined;
  components: readonly AbiParameter[];
} => {
  return input.type === "tuple";
};
