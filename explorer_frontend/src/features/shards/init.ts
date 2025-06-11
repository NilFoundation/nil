import { merge, sample } from "effector";
import { interval } from "patronum";
import { loadedPlaygroundPage } from "../code/model";
import { explorerRoute } from "../routing/routes/explorerRoute";
import { $shards, $shardsGasPriceMap, fetchShardsFx, fetchShrdsGasPriceFx } from "./models/model";

const { tick } = interval({
  timeout: 1000 * 6,
  start: merge([explorerRoute.navigated, loadedPlaygroundPage]),
  leading: true,
});

sample({
  clock: tick,
  target: fetchShardsFx,
});

sample({
  clock: tick,
  target: fetchShrdsGasPriceFx,
});

$shards.on(fetchShardsFx.doneData, (_, data) => data);
$shardsGasPriceMap.on(fetchShrdsGasPriceFx.doneData, (_, data) => data);

$shardsGasPriceMap.watch(console.log);
