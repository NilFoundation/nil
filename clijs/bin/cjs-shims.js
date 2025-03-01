import { createRequire } from 'node:module';
const { getAsset } = require('node:sea');
import path from 'node:path';

import { COMMANDS } from '../src/sea.ts'

// We use some fixed UUID to easily recognize paths from "VFS"
// from other paths in which such a UUID cannot be found.
// We will use the same prefix when putting files into assets.
// Example:
// # sea-config.json
// "assets": {
//   "/vfs-35b1b535-4fff-4ff3-882d-073f4ea7cfeb/file.cjs": "path/to/file.cjs"
// }
// # some.js
// f = require("/vfs-35b1b535-4fff-4ff3-882d-073f4ea7cfeb/file.cjs")
globalThis.VFS_PREFIX = '/vfs-35b1b535-4fff-4ff3-882d-073f4ea7cfeb';

// Similarly, we use a special identifier for the file from which
// oclif will import COMMANDS from.
// After bundling, this will be the current file, so we can just return the
// the appropriate variable as oclif expects.
globalThis.COMMANDS_FILE = '/commands-c85b2f8c-5556-4332-95b2-63ce23efe1e5.cjs';

const originalRequire = createRequire(__filename);

require = function (request) {
  const resolvedPath = path.resolve(request);

  if (resolvedPath.endsWith(COMMANDS_FILE)) {
    return { COMMANDS };
  }

  const vfsPrefixIndex = resolvedPath.indexOf(VFS_PREFIX);
  if (vfsPrefixIndex !== -1) {
    const assetName = resolvedPath.slice(vfsPrefixIndex);
    const moduleContent = getAsset(assetName, 'utf-8');
    const tempModule = { exports: {} };
    const wrapper = new Function('module', 'exports', 'require', moduleContent);
    wrapper(tempModule, tempModule.exports, originalRequire);
    return tempModule.exports;
  }

  return originalRequire(request);
};

