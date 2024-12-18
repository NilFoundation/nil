import type { fetchTransactionByHash, getChildTransactionsByHash } from "../../../api/transaction";

export type Transaction = Awaited<ReturnType<typeof fetchTransactionByHash>>;
export type TransactionChilds = Awaited<ReturnType<typeof getChildTransactionsByHash>>;

export type Currency = Transaction["currency"];
