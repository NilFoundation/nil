import type { AbiInternalType, AbiParameter } from "abitype";

export function getResultDisplay(result: unknown): string {
  if (Array.isArray(result)) {
    return result.map(getResultDisplay).join(", ");
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
