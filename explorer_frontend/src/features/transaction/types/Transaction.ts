import type { fetchTransactionByHash } from "../../../api/transaction";

export type Transaction = Awaited<ReturnType<typeof fetchTransactionByHash>>;

export type Currency = Transaction["currency"];
