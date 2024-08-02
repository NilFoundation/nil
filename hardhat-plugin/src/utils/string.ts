export function bigintReplacer(_: any, value: any) {
  return typeof value === "bigint" ? value.toString() : value;
}
