import { createDomain } from "effector";
import {
  fetchTransactionsByAddress,
  fetchTransactionsByBlock,
  getChildTransactionsByHash,
} from "../../api/transaction";

const transactionListDomain = createDomain("transaction-list");

export type TransactionFetchProps = {
  type: "block" | "address" | "transaction";
  identifier: string;
};

export type TransactionListProps = TransactionFetchProps & {
  view: "incoming" | "outgoing";
};

const createStore = transactionListDomain.createStore.bind(transactionListDomain);
const createEvent = transactionListDomain.createEvent.bind(transactionListDomain);
const createEffect = transactionListDomain.createEffect.bind(transactionListDomain);

export const $currentArguments = createStore<null | TransactionFetchProps>(null);

export const $curPage = createStore(0);

export const showList = createEvent<TransactionFetchProps>();

export const $transactionList = createStore<Awaited<ReturnType<typeof fetchTransactionsByBlock>>>(
  [],
);

export const fetchTransactionListFx = createEffect<
  TransactionFetchProps,
  Awaited<ReturnType<typeof fetchTransactionsByBlock>>
>();

fetchTransactionListFx.use(({ identifier, type }) => {
  switch (type) {
    case "block": {
      const [shard, id] = identifier.split(":");
      return fetchTransactionsByBlock(shard, id);
    }
    case "transaction": {
      return getChildTransactionsByHash(identifier);
    }
    default:
      return fetchTransactionsByAddress(identifier);
  }
});

export const nextPage = createEvent();
export const prevPage = createEvent();
