import { createDomain } from "effector";
import { fetchBlock, type fetchBlockByHash } from "../../../api/blocks";

type Block = Awaited<ReturnType<typeof fetchBlockByHash>>;

const blockDomain = createDomain();

const createStore = blockDomain.createStore.bind(blockDomain);
const createEffect = blockDomain.createEffect.bind(blockDomain);

export const $block = createStore<Block | null>(null);

export const loadBlockFx = createEffect<
  {
    shard: number;
    id: string;
  },
  Block,
  Error
>();

loadBlockFx.use(({ shard, id }) => {
  return fetchBlock(shard, id);
});
