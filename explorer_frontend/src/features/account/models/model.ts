import { createDomain } from "effector";
import { fetchAccountState } from "../../../api/account";

export const accountDomain = createDomain("account");

const createStore = accountDomain.createStore.bind(accountDomain);
const createEffect = accountDomain.createEffect.bind(accountDomain);

export const $account = createStore<AccountState | null>(null);

type AccountState = Awaited<ReturnType<typeof fetchAccountState>>;

export const loadAccountStateFx = createEffect<string, AccountState>();

loadAccountStateFx.use(async (address) => {
  return fetchAccountState(address);
});
