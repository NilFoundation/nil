import { sample } from "effector";
import { interval } from "patronum";
import { explorerRoute } from "../routing/routes/explorerRoute";
import { $latestBlocks, fetchLatestBlocksFx } from "./models/model";

const { tick } = interval({
  timeout: 1000 * 6,
  start: explorerRoute.navigated,
  stop: explorerRoute.closed,
  leading: true,
});

sample({
  clock: tick,
  target: fetchLatestBlocksFx,
});

$latestBlocks.on(fetchLatestBlocksFx.doneData, (_, list) => list);
