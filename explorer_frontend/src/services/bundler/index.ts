import * as esbuild from 'esbuild-wasm';
import { cdnResolverPlugin, } from './cdnResolverPlugin';
const initilized = false;
const esbuildVersion = '0.25.2';

const virualEntryFileName = 'index.ts';
const virtualEntryPlugin = (code: string) => ({
    name: 'virtual-entry',
    setup(build: esbuild.PluginBuild) {
        build.onResolve({ filter: /.*/ }, (args) => {
            if (args.path === virualEntryFileName) {
                return { path: args.path, namespace: 'a' };
            }
        });

        build.onLoad({ filter: /.*/, namespace: 'a' }, () => {
            return {
                contents: code,
                loader: 'tsx',
            };
        });
    },
});


export const bundle = async (
    code: string,
    injections: esbuild.Plugin[] = [],
): Promise<string> => {
    if (!initilized)  {
        await esbuild.initialize({
            wasmURL: `https://unpkg.com/esbuild-wasm@${esbuildVersion}/esbuild.wasm`,
        });
        console.log('esbuild initialized');
    }

    const build = await esbuild.build({
    entryPoints: [virualEntryFileName],
    bundle: true,
    write: false,
    plugins: [
        virtualEntryPlugin(code),
        cdnResolverPlugin(),
        ...injections,
    ],
    format: 'esm',
      target: ['es2020'],
      minify: false,
      sourcemap: 'inline',
    });

    if (build.errors.length > 0) {
        const error = build.errors[0];
        throw new Error(
            `Error: ${error.text} \nLocation: ${error.location?.file}:${error.location?.line}:${error.location?.column}`
        );
    }
    if (build.warnings.length > 0) {
        console.warn('Warnings:', build.warnings);
    }
    if (build.outputFiles.length === 0) {
        throw new Error('No output files found');
    }

    return build.outputFiles[0].text;
}