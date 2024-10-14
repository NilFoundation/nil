import { createDomain } from "effector";
import { fetchLatestBlocks } from "../../../api/transaction";

type BlocksList = Awaited<ReturnType<typeof fetchLatestBlocks>>;

export const explorerTransactionDomain = createDomain("latest-blocks");

const createStore = explorerTransactionDomain.createStore.bind(explorerTransactionDomain);
const createEffect = explorerTransactionDomain.createEffect.bind(explorerTransactionDomain);

export const $latestBlocks = createStore<BlocksList>([]);

export const fetchLatestBlocksFx = createEffect<void, BlocksList, Error>();

fetchLatestBlocksFx.use(() => {
  return fetchLatestBlocks();
});
