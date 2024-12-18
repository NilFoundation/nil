import { sample } from "effector";
import { transactionRoute } from "../routing/routes/transactionRoute";
import { $transaction, fetchTransactionFx } from "./models/transaction";
import { fetchTransactionLogsFx, $transactionLogs } from "./models/transactionLogs";
import { $transactionChilds, fetchTransactionChildsFx } from "./models/transactionChilds.ts";

sample({
  clock: transactionRoute.navigated,
  source: transactionRoute.$params,
  fn: (params) => params.hash,
  target: fetchTransactionFx,
});

// sample({
//   clock: transactionRoute.navigated,
//   source: transactionRoute.$params,
//   fn: (params) => params.hash,
//   target: fetchChildTransactionsFx,
// });

sample({
  clock: transactionRoute.navigated,
  source: transactionRoute.$params,
  fn: (params) => params.hash,
  target: fetchTransactionLogsFx,
});

$transaction.reset(transactionRoute.navigated);
$transaction.on(fetchTransactionFx.doneData, (_, transaction) => transaction);

$transactionChilds.reset(transactionRoute.navigated);
$transactionChilds.on(fetchTransactionChildsFx.doneData, (_, transactions) => transactions);

$transactionLogs.reset(transactionRoute.navigated);
$transactionLogs.on(fetchTransactionLogsFx.doneData, (_, logs) => logs);
