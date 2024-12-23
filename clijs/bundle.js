const {build} = await import('esbuild')

await build({
  bundle: true,
  entryPoints: ['./src/sea.ts'],
  format: 'cjs',
  inject: ['./bin/cjs-shims.js'],
  loader: {'.node': 'copy'},
  outfile: './dist/cli.cjs',
  platform: 'node',
  plugins: [],
  splitting: false,
  treeShaking: true,
})

await build({
  bundle: true,
  entryPoints: ['./src/commands.ts'],
  format: 'cjs',
  loader: {'.node': 'copy'},
  outfile: './dist/commands.cjs',
  platform: 'node',
  plugins: [],
  splitting: false,
  treeShaking: true,
})
