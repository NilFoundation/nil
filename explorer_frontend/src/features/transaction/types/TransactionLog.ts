import type { getTransactionLogsByHash } from "../../../api/transaction";

export type TransactionLog = Awaited<ReturnType<typeof getTransactionLogsByHash>>[number];
