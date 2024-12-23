import {Interfaces, execute} from '@oclif/core'

import pjson from '../package.json' assert {type: 'json'}

export async function run() {
  var patchedPjson = pjson as unknown as Interfaces.PJSON;
  patchedPjson.oclif.commands = {
    strategy: 'explicit',
    target: `${VFS_PREFIX}/commands.cjs`,
    identifier: 'COMMANDS',
  };

  await execute({
    loadOptions: {
      pjson: patchedPjson,
      root: __dirname,
    },
  })
}

// Needs to be anonymous function in order to run from bundled file
// eslint-disable-next-line unicorn/prefer-top-level-await
;(async () => {
  await run()
})()
