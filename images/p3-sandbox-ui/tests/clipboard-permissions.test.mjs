import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

function loadPolicy() {
    const source = readFileSync('src/app.js', 'utf8');
    const listeners = [];
    const window = {};
    const document = {
        addEventListener(type, listener) {
            listeners.push({ type, listener });
        }
    };
    vm.runInNewContext(source, { window, document });
    assert.equal(listeners.length, 1);
    assert.equal(listeners[0].type, 'DOMContentLoaded');
    return { source, policy: window.P3SandboxUi.clipboardPermissionsAttributeFor };
}

test('delegates clipboard access only to terminal tools', () => {
    const { policy } = loadPolicy();
    assert.equal(policy({ kind: 'terminal' }), 'allow="clipboard-read; clipboard-write"');
    for (const kind of ['instructions', 'ide', 'browser', undefined]) {
        assert.equal(policy({ kind }), '');
    }
    assert.equal(policy(null), '');
});

test('uses the policy for instructions, default, and lazy tool iframes', () => {
    const { source } = loadPolicy();
    assert.match(source, /clipboardPermissionsAttributeFor\(instructionsTool\)/);
    assert.match(source, /src="\$\{isDefault \? tool\.url : 'about:blank'\}"[\s\S]*clipboardPermissionsAttributeFor\(tool\)/);
    assert.equal((source.match(/clipboard-read; clipboard-write/g) || []).length, 1);
});
