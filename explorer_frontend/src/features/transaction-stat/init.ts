import { sample, merge } from "effector";
import {
  fetchTransactionsStatFx,
  $transactionsStat,
  $timeInterval,
  changeTimeInterval,
} from "./models/model";
import { explorerRoute } from "../routing/routes/explorerRoute";
import { interval } from "patronum";
import { persist } from "effector-storage/local";

const { tick } = interval({
  timeout: 1000 * 6,
  start: merge([explorerRoute.navigated, changeTimeInterval]),
  stop: merge([explorerRoute.closed, changeTimeInterval]),
  leading: true,
});

sample({
  clock: tick,
  source: $timeInterval,
  fn: (timeInterval) => timeInterval,
  target: fetchTransactionsStatFx,
});

persist({
  store: $timeInterval,
  key: "time-interval",
  sync: true,
});

$timeInterval.on(changeTimeInterval, (_, timeInterval) => timeInterval);

$transactionsStat.reset(explorerRoute.navigated);

$transactionsStat.on(fetchTransactionsStatFx.doneData, (_, data) => data);

$transactionsStat.on(changeTimeInterval, () => []);
