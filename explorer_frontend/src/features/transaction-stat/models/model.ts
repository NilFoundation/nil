import { createDomain } from "effector";
import { TimeInterval } from "../types/TimeInterval";
import { getTransactionStat } from "../../../api/transaction";

type TransactionStatData = Awaited<ReturnType<typeof getTransactionStat>>;

export const transactionStatDomain = createDomain("transactions-stat");

const createStore = transactionStatDomain.createStore.bind(transactionStatDomain);
const createEffect = transactionStatDomain.createEffect.bind(transactionStatDomain);
const createEvent = transactionStatDomain.createEvent.bind(transactionStatDomain);

export const $transactionsStat = createStore<TransactionStatData>([]);
export const $timeInterval = createStore<TimeInterval>(TimeInterval.ThirtyMinutes);

export const fetchTransactionsStatFx = createEffect<TimeInterval, TransactionStatData, Error>();
export const changeTimeInterval = createEvent<TimeInterval>();

fetchTransactionsStatFx.use((interval: TimeInterval) => {
  return getTransactionStat(interval);
});
