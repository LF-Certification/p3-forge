import assert from 'node:assert/strict';
import { copyFileSync, mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const pristine = readFileSync('src/index.html', 'utf8');
const guard = readFileSync('src/shortcut-guard.js', 'utf8').trimEnd();
const bridge = readFileSync('src/clipboard-bridge.js', 'utf8').trimEnd();
const sourceMapTrailer = '\n//# sourceMappingURL=app.c08ea5ee2370501004dc.js.map</script></body></html>';
assert.ok(pristine.endsWith(sourceMapTrailer + '\n'));
const pristineBody = pristine.slice(0, -1);
const sourceMapStart = pristineBody.lastIndexOf('\n//# sourceMappingURL=');
assert.notEqual(sourceMapStart, -1);
assert.equal(sourceMapStart, pristineBody.length - sourceMapTrailer.length);
const scriptStart = '<body><script type="text/javascript">';
const pristineScriptStart = pristine.indexOf(scriptStart) + scriptStart.length;
assert.ok(pristineScriptStart >= scriptStart.length);

function generate() {
    const directory = mkdtempSync(join(tmpdir(), 'p3-terminal-index-'));
    const indexPath = join(directory, 'index.html');
    copyFileSync('src/index.html', indexPath);
    const result = spawnSync(process.execPath, [
        'scripts/inject-custom-scripts.mjs', indexPath,
        'src/shortcut-guard.js', 'src/clipboard-bridge.js'
    ], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return readFileSync(indexPath, 'utf8');
}

function markedSource(index, name) {
    const startMarker = `/* P3_${name}_START */\n`;
    const endMarker = `\n/* P3_${name}_END */`;
    const start = index.indexOf(startMarker);
    const end = index.indexOf(endMarker, start);
    assert.notEqual(start, -1);
    assert.notEqual(end, -1);
    return { start, end, source: index.slice(start + startMarker.length, end) };
}

test('injects canonical integrations inside the ttyd application entrypoint', () => {
    const generated = generate();
    const guardBlock = markedSource(generated, 'SHORTCUT_GUARD');
    const bridgeBlock = markedSource(generated, 'CLIPBOARD_BRIDGE');
    const insertionStart = generated.indexOf('/* P3_SHORTCUT_GUARD_START */');
    const entrypointAnchor = 'i(423),(0,e.render)';
    const entrypointStart = pristine.indexOf(entrypointAnchor);
    const bridgeEnd = bridgeBlock.end + '\n/* P3_CLIPBOARD_BRIDGE_END */'.length;

    assert.equal(generated.slice(0, insertionStart), pristine.slice(0, entrypointStart) + 'i(423);');
    assert.equal(generated.slice(bridgeEnd), '\n(0,e.render)' + pristine.slice(entrypointStart + entrypointAnchor.length));
    assert.equal(generated.match(/i\(423\);/g)?.length, 1);
    assert.doesNotMatch(generated, /i\(423\),\(0,e\.render\)/);
    assert.equal(guardBlock.source, guard);
    assert.equal(bridgeBlock.source, bridge);
    assert.ok(guardBlock.end < bridgeBlock.start);
    assert.equal(generated.match(/P3_SHORTCUT_GUARD_START/g)?.length, 1);
    assert.equal(generated.match(/P3_CLIPBOARD_BRIDGE_START/g)?.length, 1);
    assert.doesNotMatch(`${guardBlock.source}\n${bridgeBlock.source}`, /beforeunload/i);
    assert.equal(generated.match(/<script\b/gi)?.length, 1);
    assert.ok(generated.endsWith(sourceMapTrailer + '\n'));
    assert.equal(generated.lastIndexOf('\n//# sourceMappingURL='), generated.indexOf('\n//# sourceMappingURL='));
    assert.equal(generate(), generated);
});

test('rejects reinjecting an already generated index', () => {
    const directory = mkdtempSync(join(tmpdir(), 'p3-terminal-index-'));
    const indexPath = join(directory, 'index.html');
    copyFileSync('src/index.html', indexPath);
    const args = [
        'scripts/inject-custom-scripts.mjs', indexPath,
        'src/shortcut-guard.js', 'src/clipboard-bridge.js'
    ];
    assert.equal(spawnSync(process.execPath, args).status, 0);
    const second = spawnSync(process.execPath, args, { encoding: 'utf8' });
    assert.notEqual(second.status, 0);
    assert.match(second.stderr, /already contains P3 terminal integrations/);
});
