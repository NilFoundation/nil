import { sample } from "effector";
import {
  $curPage,
  $currentArguments,
  $transactionList,
  fetchTransactionListFx,
  showList,
} from "./model";

sample({
  clock: sample({
    clock: showList,
    source: $currentArguments,
    filter: (args, props) => {
      return props === null || args?.type !== props.type || args?.identifier !== props.identifier;
    },
    fn: (_, props) => props,
  }),
  target: fetchTransactionListFx,
});

$currentArguments.on(fetchTransactionListFx, (_, props) => props);

$curPage.reset($currentArguments);

$transactionList.on(fetchTransactionListFx.doneData, (_, transactions) => transactions);
