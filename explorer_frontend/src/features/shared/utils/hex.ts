const hexRegex = /^[0-9A-Fa-f]+$/;

export const isHex = (value: string): boolean => {
  return hexRegex.test(value);
};

export const removeHexPrefix = (str: `0x${string}` | string): string => {
  return str.startsWith("0x") ? str.slice(2) : str;
};

export const addHexPrefix = (str: `0x${string}` | string): `0x${string}` => {
  return `0x${removeHexPrefix(str)}`;
};
