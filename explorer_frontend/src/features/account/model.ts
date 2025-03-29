import { type CometaClient, type Hex, addHexPrefix } from "@nilfoundation/niljs";
import { createDomain } from "effector";
import { fetchAccountState } from "../../api/account";
import type { AccountCometaInfo, AccountState } from "./types";

const accountDomain = createDomain("account");

const createStore = accountDomain.createStore.bind(accountDomain);
const createEffect = accountDomain.createEffect.bind(accountDomain);

export const $account = createStore<AccountState | null>(null);
export const $accountCometaInfo = createStore<AccountCometaInfo | null>(null);

export const loadAccountStateFx = createEffect<string, AccountState>();
export const loadAccountCometaInfoFx = createEffect<
  {
    address: Hex;
    cometaClient: CometaClient;
  },
  AccountCometaInfo
>();

loadAccountStateFx.use(async (address) => {
  return fetchAccountState(address);
});

loadAccountCometaInfoFx.use(async ({ address, cometaClient }) => {
  const res = await cometaClient.getContract(addHexPrefix(address));
  return res;
});
