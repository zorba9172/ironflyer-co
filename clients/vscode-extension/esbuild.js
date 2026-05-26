// Bundles the extension into a single CommonJS file under dist/.
// VSCode loads dist/extension.js; everything else (node_modules, src, media
// referenced from the webview) is resolved at runtime.

const esbuild = require('esbuild');

const production = process.argv.includes('--production');
const watch = process.argv.includes('--watch');

/** @type {import('esbuild').BuildOptions} */
const opts = {
  entryPoints: ['src/extension.ts'],
  bundle: true,
  outfile: 'dist/extension.js',
  external: ['vscode'],
  format: 'cjs',
  platform: 'node',
  target: 'node18',
  sourcemap: !production,
  minify: production,
  logLevel: 'info',
};

async function main() {
  if (watch) {
    const ctx = await esbuild.context(opts);
    await ctx.watch();
    console.log('esbuild watching…');
  } else {
    await esbuild.build(opts);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
