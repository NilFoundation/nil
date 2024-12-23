import { createRequire } from 'node:module';
const { getAsset } = require('node:sea');
import path from 'node:path';

// We use some fixed UUID to easily recognize paths from "VFS"
// from other paths in which such a UUID cannot be found.
// We will use the same prefix when putting files into assets.
globalThis.VFS_PREFIX = '/vfs-35b1b535-4fff-4ff3-882d-073f4ea7cfeb';

const originalRequire = createRequire(__filename);

require = function (request) {
  const resolvedPath = path.resolve(request);

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

