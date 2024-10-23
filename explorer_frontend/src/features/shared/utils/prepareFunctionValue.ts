import type { InputParameter } from "../../../types";

export enum SimpleType {
  uint = "uint",
  int = "int",
  address = "address",
  bool = "bool",
  bytes = "bytes",
  string = "string",
  array = "array",
}

export const mappingType = (inputType: string): SimpleType => {
  if (inputType.includes("uint")) {
    return SimpleType.uint;
  }
  if (inputType.includes("int")) {
    return SimpleType.int;
  }
  if (inputType.includes("address")) {
    return SimpleType.address;
  }
  if (inputType.includes("bool")) {
    return SimpleType.bool;
  }
  return SimpleType.string;
};

export const prepareFunctionValue = (simpleType: SimpleType, value: string) => {
  switch (simpleType) {
    case SimpleType.uint:
    case SimpleType.int:
      return Number.parseInt(value);
    case SimpleType.address:
      return value;
    case SimpleType.bool:
      return value === "true";
    case SimpleType.string:
      return value;
  }
};

export const prepareValuesByFunction = (
  inputs: InputParameter[],
  values: Record<string, string>,
) => {
  const mappedTypes: [string, ReturnType<typeof prepareFunctionValue>][] = inputs.map((input) => {
    const simpleType = mappingType(input.type);
    return [input.name, prepareFunctionValue(simpleType, values[input.name])];
  });
  return mappedTypes.reduce((acc, [name, value]) => {
    acc[name] = value;
    return acc;
  }, {});
};
