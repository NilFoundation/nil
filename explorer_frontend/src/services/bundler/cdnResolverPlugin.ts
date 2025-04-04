import type * as esbuild from 'esbuild';
const cdnUrl = 'https://esm.sh/';

export function cdnResolverPlugin(): esbuild.Plugin {
  return {
    name: 'cdn-resolver',
    setup(build: esbuild.PluginBuild) {
      // Handle bare imports (npm package names)

      build.onResolve({ filter: /^[^./]|^\.[^./]|^\.\.[^/]/ }, async (args) => {
        // Skip Node.js built-ins
        if (args.path.match(/^(node:)?fs|path|crypto|buffer|stream|zlib|util|os|child_process|worker_threads|cluster$/)) {
          return { external: true };
        }

        if (args.path.startsWith('/-/')) {
            const resolved = new URL(args.path.replace('/-/', `${cdnUrl}pin/`), cdnUrl);
            console.log('Resolved URL:', resolved);
            return { path: resolved, namespace: 'unpkg' };
        }
        
        // Handle package paths
        let url: string;
        if (args.path.includes('/')) {
            console.log('Args:', args);
            
          // This is an import like 'lodash/get' or '@material-ui/core/Button'
          url = `${cdnUrl}${args.path}`;
        } else {
          // This is a bare import like 'lodash' or 'react'
          url = `${cdnUrl}${args.path}`;
        }
        
        return { path: url, namespace: 'unpkg' };
      });

      build.onResolve({ filter: /^\// }, async (args) => {
        // Handle package paths
        const url = `${cdnUrl}${args.path.slice(1)}`;
        
        return { path: url, namespace: 'unpkg' };
      });
      
      // Handle relative imports within packages
      build.onResolve({ filter: /^\./, namespace: 'unpkg' }, (args) => {
        // Skip data: URIs
        if (args.path.startsWith('data:')) {
          return { external: true };
        }
        
        // Handle https:// URLs directly
        if (args.path.startsWith('https://')) {
          return { path: args.path, namespace: 'unpkg' };
        }
        
        // Handle relative imports
        if (args.path.startsWith('./') || args.path.startsWith('../')) {
            console.log('Args:', args);
          // Get the directory of the importer
          const importerUrl = new URL(args.importer);
          const baseUrl = importerUrl.href.endsWith('.js')? importerUrl.href.substring(0, importerUrl.href.lastIndexOf('/') + 1) : importerUrl.href;

          console.log('Base URL:', baseUrl);
            console.log('Importer URL:', importerUrl.href);
          
          // Resolve the relative path
          const resolved = new URL(args.path, baseUrl).href;
            console.log('Resolved URL:', resolved);
          return { path: resolved, namespace: 'unpkg' };
        }
        
        // Handle absolute paths within the package
        if (args.path.startsWith('/')) {
          const importerUrl = new URL(args.importer);
          const packageRoot = `https://${importerUrl.host}`;
          const resolved = new URL(args.path, packageRoot).href;
          return { path: resolved, namespace: 'unpkg' };
        }
        
        // For any other import pattern, assume it's a new package
        return { path: `${cdnUrl}${args.path}`, namespace: 'unpkg' };
      });
      
      // Load files from unpkg
      build.onLoad({ filter: /.*/, namespace: 'unpkg' }, async (args) => {
        try {
          const response = await fetch(args.path);
          
          if (!response.ok) {
            throw new Error(`Failed to fetch ${args.path}: ${response.status} ${response.statusText}`);
          }
          
          const contents = await response.text();
          
          // Determine the loader based on file extension
          const url = new URL(args.path);
          const path = url.pathname;
          let loader = 'js';
          
          if (path.endsWith('.json')) {
            loader = 'json';
          } else if (path.endsWith('.css')) {
            loader = 'css';
          } else if (path.endsWith('.jsx') || path.endsWith('.tsx')) {
            loader = 'jsx';
          } else if (path.endsWith('.ts')) {
            loader = 'ts';
          }
          
          return {
            contents,
            loader,
          };
        } catch (error) {
          return {
            errors: [
              {
                text: `Error loading ${args.path}: ${error.message}`,
                location: { file: args.path },
              },
            ],
          };
        }
      });
    },
  };
}