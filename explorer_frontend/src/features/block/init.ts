import { combine, sample } from "effector";
import { blockDetailsRoute, blockRoute } from "../routing/routes/blockRoute";
import { $block, loadBlockFx } from "./models/model";

sample({
  clock: blockRoute.navigated,
  source: blockRoute.$params,
  filter: combine(blockRoute.$params, $block, (params, block) => {
    if (!block) return true;
    return block.id !== params.id || block.shard_id !== +params.shard;
  }),
  fn: (params) => ({
    shard: +params.shard,
    id: params.id,
  }),
  target: loadBlockFx,
});

sample({
  clock: blockDetailsRoute.navigated,
  source: blockDetailsRoute.$params,
  filter: combine(blockDetailsRoute.$params, $block, (params, block) => {
    if (!block) return true;
    return block.id !== params.id || block.shard_id !== +params.shard;
  }),
  fn: (params) => ({
    shard: +params.shard,
    id: params.id,
  }),
  target: loadBlockFx,
});

$block.on(loadBlockFx.doneData, (_, block) => block);
