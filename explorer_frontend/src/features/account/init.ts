import { sample } from "effector";
import { addressRoute } from "../routing";
import { $account, loadAccountStateFx } from "./models/model";

sample({
  clock: addressRoute.navigated,
  source: addressRoute.$params,
  fn: (params) => params.address,
  target: loadAccountStateFx,
});

$account.reset(addressRoute.navigated);
$account.on(loadAccountStateFx.doneData, (_, account) => account);
