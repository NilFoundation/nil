import { createDomain } from "effector";
import { getTransactionLogsByHash } from "../../../api/transaction";
import type { TransactionLog } from "../types/TransactionLog";

export const explorerTransactionLogsDomain = createDomain("explorer-transaction-logs");

const createStore = explorerTransactionLogsDomain.createStore.bind(explorerTransactionLogsDomain);
const createEffect = explorerTransactionLogsDomain.createEffect.bind(explorerTransactionLogsDomain);

export const $transactionLogs = createStore<TransactionLog[]>([]);

export const fetchTransactionLogsFx = createEffect<string, TransactionLog[]>();
fetchTransactionLogsFx.use(getTransactionLogsByHash);
