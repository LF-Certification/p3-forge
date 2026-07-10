import { readFileSync, writeFileSync } from 'node:fs';

const [indexPath, shortcutGuardPath, clipboardBridgePath] = process.argv.slice(2);
if (!indexPath || !shortcutGuardPath || !clipboardBridgePath) {
    throw new Error('usage: inject-custom-scripts.mjs INDEX SHORTCUT_GUARD CLIPBOARD_BRIDGE');
}

const entrypointAnchor = 'i(423),(0,e.render)';
const index = readFileSync(indexPath, 'utf8');
const shortcutGuard = readFileSync(shortcutGuardPath, 'utf8').trimEnd();
const clipboardBridge = readFileSync(clipboardBridgePath, 'utf8').trimEnd();

if (index.includes('P3_SHORTCUT_GUARD_START') || index.includes('P3_CLIPBOARD_BRIDGE_START')) {
    throw new Error(`${indexPath} already contains P3 terminal integrations`);
}
if (index.indexOf(entrypointAnchor) === -1 || index.indexOf(entrypointAnchor) !== index.lastIndexOf(entrypointAnchor)) {
    throw new Error(`expected ${indexPath} to contain the pinned ttyd entrypoint once`);
}
for (const [name, source] of [['shortcut guard', shortcutGuard], ['clipboard bridge', clipboardBridge]]) {
    if (/<\/script/i.test(source)) {
        throw new Error(`${name} must not contain a closing script tag`);
    }
}

// Inject into ttyd's executed application entrypoint. The custom-index runtime
// retains statements outside this entrypoint in the DOM without evaluating
// them, so sibling, prefixed, and appended integrations are ineffective.
const insertion = [
    '/* P3_SHORTCUT_GUARD_START */',
    shortcutGuard,
    '/* P3_SHORTCUT_GUARD_END */',
    '/* P3_CLIPBOARD_BRIDGE_START */',
    clipboardBridge,
    '/* P3_CLIPBOARD_BRIDGE_END */'
].join('\n');

writeFileSync(indexPath, index.replace(entrypointAnchor, `i(423);${insertion}\n(0,e.render)`));
