// Test-only observation: never changes Pi's tool, prompt, or RPC behavior.
const fs = require('node:fs');
const cp = require('node:child_process');
const { syncBuiltinESMExports } = require('node:module');
const path = require('node:path');
const mark = (kind, data = {}) => fs.appendFileSync(process.env.GCM_NATIVE_EVENTS,
  JSON.stringify({ kind, pid: process.pid, ...data }) + '\n');
mark('preload', { argv: process.argv.slice(2) });
process.on('exit', code => mark('exit', { code }));

const fetch = globalThis.fetch;
globalThis.fetch = function(input, ...args) {
  const url = new URL(typeof input === 'string' || input instanceof URL ? input : input.url);
  mark('fetch', { url: url.href });
  if (url.origin !== process.env.GCM_NATIVE_ORIGIN) {
    mark('external-fetch', { url: url.href });
    return Promise.reject(new Error('Native fixture blocks external fetch'));
  }
  return fetch.call(this, input, ...args);
};
// Prove the installed instrumentation is active without contacting any server.
void globalThis.fetch('https://native-fixture.invalid/preload-check').catch(() => mark('network-check'));

for (const name of ['spawn', 'spawnSync']) {
  const original = cp[name];
  cp[name] = function(command, args, ...rest) {
    const helper = path.basename(String(command));
    const observed = ['rg', 'fd', 'fdfind'].includes(helper);
    if (observed) mark('helper', { helper, args, command });
    const result = original.call(this, command, args, ...rest);
    if (observed && result.pid) mark('helper-pid', { child: result.pid });
    return result;
  };
}
syncBuiltinESMExports();

if (process.argv.includes('--version')) {
  let version = '';
  const write = process.stdout.write;
  process.stdout.write = function(chunk, ...args) {
    version += chunk.toString();
    return write.call(this, chunk, ...args);
  };
  process.on('exit', () => mark('version', { version: version.trim() }));
}

if (process.argv.includes('--mode')) {
  let input = '', output = '';
  const emit = process.stdin.emit;
  process.stdin.emit = function(event, ...args) {
    if (event === 'data') {
      input += args[0].toString();
      let end;
      while ((end = input.indexOf('\n')) >= 0) {
        const line = input.slice(0, end); input = input.slice(end + 1);
        if (line) mark('command', { record: JSON.parse(line) });
      }
    }
    if (event === 'end') mark('stdin-eof');
    return emit.call(this, event, ...args);
  };
  const write = process.stdout.write;
  process.stdout.write = function(chunk, ...args) {
    output += chunk.toString();
    let end;
    while ((end = output.indexOf('\n')) >= 0) {
      const line = output.slice(0, end); output = output.slice(end + 1);
      if (line) mark('event', { record: JSON.parse(line) });
    }
    return write.call(this, chunk, ...args);
  };
}
