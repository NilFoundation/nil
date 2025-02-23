export enum ExtensionMethods {
  eth_sendTransaction = "eth_sendTransaction",
  eth_requestAccounts = "eth_requestAccounts",
}

export function isExtensionMethod(method: string): boolean {
  return Object.keys(ExtensionMethods).includes(method);
}
