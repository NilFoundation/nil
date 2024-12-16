import { sample } from "effector";
import { addressRoute } from "../routing";
import { $account, $accountCometaInfo, loadAccountCometaInfoFx, loadAccountStateFx } from "./model";
import { combine } from "effector";
import { $cometaService } from "../cometa/model";

sample({
  clock: addressRoute.navigated,
  source: addressRoute.$params,
  fn: (params) => params.address,
  target: loadAccountStateFx,
});

sample({
  clock: addressRoute.navigated,
  source: combine(addressRoute.$params, $cometaService, (params, cometaService) => ({
    params,
    cometaService,
  })),
  filter: ({ cometaService }) => cometaService !== null,
  fn: (params) => params.address,
  target: loadAccountCometaInfoFx,
});

$accountCometaInfo.reset(addressRoute.navigated);
$accountCometaInfo.on(loadAccountCometaInfoFx.doneData, (_, info) => info);
$account.reset(addressRoute.navigated);
$account.on(loadAccountStateFx.doneData, (_, account) => account);
