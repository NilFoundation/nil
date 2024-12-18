import { createDomain } from "effector";
import { getChildTransactionsByHash } from "../../../api/transaction";
import type { TransactionChilds } from "../types/Transaction.ts";

export const explorerTransactionChildsDomain = createDomain("explorer-transaction-childs");

const createStore = explorerTransactionChildsDomain.createStore.bind(
  explorerTransactionChildsDomain,
);
const createEffect = explorerTransactionChildsDomain.createEffect.bind(
  explorerTransactionChildsDomain,
);

export const $transactionChilds = createStore<TransactionChilds[]>([]);

export const fetchTransactionChildsFx = createEffect<string, TransactionChilds[]>();
fetchTransactionChildsFx.use(getChildTransactionsByHash);
