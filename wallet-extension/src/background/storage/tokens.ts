export interface Token {
  name: string;
  address: string;
  show: boolean;
  topupable: boolean;
}

const STORAGE_KEY = "nil_tokens";

export const saveToken = async (token: Token): Promise<void> => {
  const result = await chrome.storage.local.get(STORAGE_KEY);
  const existingTokens: Token[] = result[STORAGE_KEY] || [];
  const otherTokens = existingTokens.filter((t) => t.address !== token.address);
  const updatedTokens = [...otherTokens, token];
  await chrome.storage.local.set({ [STORAGE_KEY]: updatedTokens });
};

export const setTokens = async (tokens: Token[]): Promise<void> => {
  await chrome.storage.local.set({ [STORAGE_KEY]: tokens });
};

export const getTokens = async (): Promise<Token[]> => {
  const result = await chrome.storage.local.get(STORAGE_KEY);
  return result[STORAGE_KEY] || [];
};
