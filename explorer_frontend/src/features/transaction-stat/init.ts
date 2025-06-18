import { merge, sample } from "effector";
import { persist } from "effector-storage/local";
import { interval } from "patronum";
import { explorerRoute } from "../routing/routes/explorerRoute";
import {
  $timeInterval,
  $transactionsStat,
  changeTimeInterval,
  fetchTransactionsStatFx,
} from "./models/model";

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

$transactionsStat.on(fetchTransactionsStatFx.doneData, (_, data) => data);

$transactionsStat.on(changeTimeInterval, () => []);
$transactionsStat.watch(console.log)
