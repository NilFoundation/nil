import { type Command, type Interfaces, execute } from "@oclif/core";

import pjson from "../package.json" assert { type: "json" };

import Keygen from "./commands/keygen/index.js";
import KeygenNewP2p from "./commands/keygen/new-p2p.js";
import KeygenNew from "./commands/keygen/new.js";

import AbiCommand from "./commands/abi";
import AbiDecode from "./commands/abi/decode";
import AbiEncode from "./commands/abi/encode";
import BlockCommand from "./commands/block";
import SmartAccountBalance from "./commands/smart-account/balance.js";
import SmartAccountCallReadOnly from "./commands/smart-account/call-readonly";
import SmartAccountDeploy from "./commands/smart-account/deploy.js";
import SmartAccountEstimateFee from "./commands/smart-account/estimate-fee";
import SmartAccount from "./commands/smart-account/index.js";
import SmartAccountInfo from "./commands/smart-account/info.js";
import SmartAccountNew from "./commands/smart-account/new.js";
import SmartAccountSendToken from "./commands/smart-account/send-tokens";
import SmartAccountSendTransaction from "./commands/smart-account/send-transaction.js";
import SmartAccountSeqno from "./commands/smart-account/seqno";
import SmartAccountTopup from "./commands/smart-account/top-up";

export const COMMANDS: Record<string, Command.Class> = {
  abi: AbiCommand,
  "abi:decode": AbiDecode,
  "abi:encode": AbiEncode,

  block: BlockCommand,

  keygen: Keygen,
  "keygen:new": KeygenNew,
  "keygen:new-p2p": KeygenNewP2p,

  "smart-account": SmartAccount,
  "smart-account:balance": SmartAccountBalance,
  "smart-account:call-readonly": SmartAccountCallReadOnly,
  "smart-account:deploy": SmartAccountDeploy,
  "smart-account:estimate-fee": SmartAccountEstimateFee,
  "smart-account:info": SmartAccountInfo,
  "smart-account:new": SmartAccountNew,
  "smart-account:send-tokens": SmartAccountSendToken,
  "smart-account:send-transaction": SmartAccountSendTransaction,
  "smart-account:seqno": SmartAccountSeqno,
  "smart-account:top-up": SmartAccountTopup,
};

export async function run() {
  const patchedPjson = pjson as unknown as Interfaces.PJSON;
  patchedPjson.oclif.commands = {
    strategy: "explicit",
    target: COMMANDS_FILE,
    identifier: "COMMANDS",
  };

  await execute({
    loadOptions: {
      pjson: patchedPjson,
      root: __dirname,
    },
  });
}
// Needs to be anonymous function in order to run from bundled file
// eslint-disable-next-line unicorn/prefer-top-level-await
(async () => {
  await run();
})();
