import type { fetchAccountState } from "../../api/account";

export type AccountState = Awaited<ReturnType<typeof fetchAccountState>>;

export type AccountCometaInfo = {
  sourceCode?: string;
};
