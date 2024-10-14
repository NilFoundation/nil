import { createDomain } from "effector";
import { fetchTransactionByHash } from "../../../api/transaction";
import type { Transaction } from "../types/Transaction";

export const explorerTransactionDomain = createDomain("explorer-transaction");

const createStore = explorerTransactionDomain.createStore.bind(explorerTransactionDomain);
const createEffect = explorerTransactionDomain.createEffect.bind(explorerTransactionDomain);

export const $transaction = createStore<Transaction | null>(null);

export const fetchTransactionFx = createEffect<string, Transaction>();
fetchTransactionFx.use(fetchTransactionByHash);
