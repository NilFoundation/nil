import { sample } from "effector";
import { fetchLatestBlocksFx, $latestBlocks } from "./models/model";
import { interval } from "patronum";
import { explorerRoute } from "../routing/routes/explorerRoute";

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

$latestBlocks.reset(explorerRoute.navigated);

$latestBlocks.on(fetchLatestBlocksFx.doneData, (_, list) => list);
