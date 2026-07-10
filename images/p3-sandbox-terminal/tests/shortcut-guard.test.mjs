import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function loadGuard(path) {
    const listeners = [];
    const window = {
        addEventListener(type, listener, options) {
            listeners.push({ type, listener, options });
        }
    };
    const document = {};
    window.window = window;
    window.document = document;

    vm.runInNewContext(readFileSync(path, 'utf8'), {
        window,
        document,
        Set,
        console
    });

    assert.equal(listeners.length, 1, `${path} should install one keydown listener`);
    assert.equal(listeners[0].type, 'keydown');
    assert.equal(listeners[0].options, true, `${path} should use capture phase`);
    assert.equal(typeof window.P3BrowserShortcutGuard.shouldCancelBrowserShortcut, 'function');
    return { listener: listeners[0].listener, shouldCancel: window.P3BrowserShortcutGuard.shouldCancelBrowserShortcut };
}

function keyEvent(overrides = {}) {
    let prevented = false;
    let stopped = false;
    return {
        key: '',
        ctrlKey: false,
        metaKey: false,
        altKey: false,
        defaultPrevented: false,
        target: {},
        preventDefault() {
            prevented = true;
            this.defaultPrevented = true;
        },
        stopPropagation() {
            stopped = true;
        },
        get prevented() {
            return prevented;
        },
        get stopped() {
            return stopped;
        },
        ...overrides
    };
}

for (const path of [
    'src/shortcut-guard.js',
    '../p3-sandbox-ui/src/shortcut-guard.js'
]) {
    const { listener, shouldCancel } = loadGuard(path);

    for (const key of ['s', 'S', 'f', 'p', 'u']) {
        const event = keyEvent({ key, ctrlKey: true });
        listener(event);
        assert.equal(event.prevented, true, `${path} should cancel Ctrl+${key}`);
        assert.equal(event.stopped, false, `${path} must not stop propagation for Ctrl+${key}`);
    }

    const commandSave = keyEvent({ key: 's', metaKey: true });
    listener(commandSave);
    assert.equal(commandSave.prevented, true, `${path} should cancel Cmd+S`);
    assert.equal(commandSave.stopped, false, `${path} must not stop propagation for Cmd+S`);

    const f1 = keyEvent({ key: 'F1' });
    listener(f1);
    assert.equal(f1.prevented, true, `${path} should cancel F1`);

    const backspace = keyEvent({ key: 'Backspace' });
    listener(backspace);
    assert.equal(backspace.prevented, true, `${path} should cancel backspace outside editable controls`);

    const editableBackspace = keyEvent({
        key: 'Backspace',
        target: { isContentEditable: false, tagName: 'INPUT', getAttribute: () => 'text' }
    });
    assert.equal(shouldCancel(editableBackspace), false, `${path} should allow backspace in editable inputs`);

    const altCtrlF = keyEvent({ key: 'f', ctrlKey: true, altKey: true });
    listener(altCtrlF);
    assert.equal(altCtrlF.prevented, false, `${path} should not cancel Alt+Ctrl+F`);
}


const terminalEntrypoint = readFileSync('entrypoint.sh', 'utf8');
assert.match(terminalEntrypoint, /ttyd -I \/usr\/share\/ttyd\/index\.html/, 'entrypoint should serve the custom ttyd index');
assert.match(terminalEntrypoint, /-t disableLeaveAlert=true/, 'entrypoint should disable ttyd nested-frame leave alert');

const terminalDockerfile = readFileSync('Dockerfile', 'utf8');
assert.match(terminalDockerfile, /inject-custom-scripts\.mjs/, 'terminal image should generate its custom ttyd index');

const uiIndex = readFileSync('../p3-sandbox-ui/src/index.html', 'utf8');
assert.match(uiIndex, /<script src="shortcut-guard\.js"><\/script>\s*<script src="app\.js"><\/script>/, 'sandbox-ui should load guard before app.js');

const uiApp = readFileSync('../p3-sandbox-ui/src/app.js', 'utf8');
assert.doesNotMatch(uiApp, /BROWSER_SHORTCUT_CTRL_KEYS/, 'sandbox-ui app should not duplicate the shared guard');
