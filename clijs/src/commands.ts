import { Command } from '@oclif/core'

import Keygen from './commands/keygen/index.ts'
import KeygenNew from './commands/keygen/new.ts'
import KeygenNewP2p from './commands/keygen/new-p2p.ts'

import Wallet from './commands/wallet/index.ts'
import WalletNew from './commands/wallet/new.ts'
import WalletBalance from './commands/wallet/balance.ts'

export const COMMANDS: Record<string, Command.Class> = {
    keygen: Keygen,
    'keygen:new': KeygenNew,
    'keygen:new-p2p': KeygenNewP2p,

    wallet: Wallet,
    'wallet:new': WalletNew,
    'wallet:balance': WalletBalance,
}

