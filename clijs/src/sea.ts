import { type Command, type Interfaces, execute } from "@oclif/core";

import oclifrc from "../.oclifrc.json" assert { type: "json" };
import pjson from "../package.json" assert { type: "json" };

import Keygen from "./commands/keygen/index.js";
import KeygenNewP2p from "./commands/keygen/new-p2p.js";
import KeygenNew from "./commands/keygen/new.js";

import AbiCommand from "./commands/abi";
import AbiDecode from "./commands/abi/decode";
import AbiEncode from "./commands/abi/encode";
import BlockCommand from "./commands/block";
import ConfigGet from "./commands/config/get";
import ConfigInit from "./commands/config/init";
import ConfigSet from "./commands/config/set";
import ConfigShow from "./commands/config/show";
import Contract from "./commands/contract";
import ContractAddress from "./commands/contract/address";
import ContractBalance from "./commands/contract/balance";
import ContractCallReadOnly from "./commands/contract/call-readonly";
import ContractCode from "./commands/contract/code";
import ContractDeploy from "./commands/contract/deploy";
import ContractEstimateFee from "./commands/contract/estimate-fee";
import ContractTokens from "./commands/contract/tokens";
import MinterCommand from "./commands/minter";
import MinterBurnTokenCommand from "./commands/minter/burn";
import MinterCreateTokenCommand from "./commands/minter/create";
import MinterMintTokenCommand from "./commands/minter/mint";
import ReceiptCommand from "./commands/receipt";
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
import SystemCommand from "./commands/system";
import ChainId from "./commands/system/chain-id";
import GasPrice from "./commands/system/gas-price";
import Shards from "./commands/system/shards";
import UtilCommand from "./commands/util";
import ListCommands from "./commands/util/list-commands";

export const COMMANDS: Record<string, Command.Class> = {
  abi: AbiCommand,
  "abi:decode": AbiDecode,
  "abi:encode": AbiEncode,

  block: BlockCommand,

  "config:get": ConfigGet,
  "config:init": ConfigInit,
  "config:set": ConfigSet,
  "config:show": ConfigShow,

  contract: Contract,
  "contract:address": ContractAddress,
  "contract:balance": ContractBalance,
  "contract:call-readonly": ContractCallReadOnly,
  "contract:code": ContractCode,
  "contract:deploy": ContractDeploy,
  "contract:estimate-fee": ContractEstimateFee,
  "contract:seqno": SmartAccountSeqno,
  "contract:tokens": ContractTokens,
  "contract:top-up": SmartAccountTopup,

  keygen: Keygen,
  "keygen:new": KeygenNew,
  "keygen:new-p2p": KeygenNewP2p,

  minter: MinterCommand,
  "minter:create": MinterCreateTokenCommand,
  "minter:burn": MinterBurnTokenCommand,
  "minter:mint": MinterMintTokenCommand,

  receipt: ReceiptCommand,

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

  system: SystemCommand,
  "system:chain-id": ChainId,
  "system:gas-price": GasPrice,
  "system:shards": Shards,

  util: UtilCommand,
  "util:list-commands": ListCommands,
};

export async function run() {
  const patchedPjson = pjson as unknown as Interfaces.PJSON;
  patchedPjson.oclif = oclifrc;
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
