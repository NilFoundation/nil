import { type Command, type Interfaces, execute } from "@oclif/core";

import pjson from "../package.json" assert { type: "json" };

import Keygen from "./commands/keygen/index.js";
import KeygenNewP2p from "./commands/keygen/new-p2p.js";
import KeygenNew from "./commands/keygen/new.js";

import SmartAccountBalance from "./commands/smart-account/balance.js";
import SmartAccountDeploy from "./commands/smart-account/deploy.js";
import SmartAccount from "./commands/smart-account/index.js";
import SmartAccountInfo from "./commands/smart-account/info.js";
import SmartAccountNew from "./commands/smart-account/new.js";
import SmartAccountSendTransaction from "./commands/smart-account/send-transaction.js";

export const COMMANDS: Record<string, Command.Class> = {
  keygen: Keygen,
  "keygen:new": KeygenNew,
  "keygen:new-p2p": KeygenNewP2p,

  "smart-account": SmartAccount,
  "smart-account:balance": SmartAccountBalance,
  "smart-account:deploy": SmartAccountDeploy,
  "smart-account:info": SmartAccountInfo,
  "smart-account:new": SmartAccountNew,
  "smart-account:send-transaction": SmartAccountSendTransaction,
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
