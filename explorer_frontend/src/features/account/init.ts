import { sample } from "effector";
import { combine } from "effector";
import { $cometaClient } from "../cometa/model";
import { addressRoute } from "../routing";
import { $account, $accountCometaInfo, loadAccountCometaInfoFx, loadAccountStateFx } from "./model";

sample({
  clock: addressRoute.navigated,
  source: addressRoute.$params,
  fn: (params) => params.address,
  target: loadAccountStateFx,
});

sample({
  clock: addressRoute.navigated,
  source: combine(addressRoute.$params, $cometaClient, (params, cometaClient) => ({
    params,
    cometaClient,
  })),
  filter: ({ cometaClient }) => cometaClient !== null,
  fn: (params) => params.address,
  target: loadAccountCometaInfoFx,
});

$accountCometaInfo.reset(addressRoute.navigated);
$accountCometaInfo.on(loadAccountCometaInfoFx.doneData, (_, info) => info);
$account.reset(addressRoute.navigated);
$account.on(loadAccountStateFx.doneData, (_, account) => account);
