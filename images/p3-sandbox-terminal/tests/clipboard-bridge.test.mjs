import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync('src/clipboard-bridge.js', 'utf8');

function eventTarget(parent = null) {
    const listeners = new Map();
    return {
        parent,
        addEventListener(type, listener, options) {
            const handlers = listeners.get(type) || [];
            handlers.push({ listener, options });
            listeners.set(type, handlers);
        },
        removeEventListener(type, listener) {
            const handlers = listeners.get(type) || [];
            listeners.set(type, handlers.filter(handler => handler.listener !== listener));
        },
        contains(target) {
            for (let current = target; current; current = current.parent) {
                if (current === this) return true;
            }
            return false;
        },
        listenerOptions(type) {
            return (listeners.get(type) || []).map(handler => handler.options);
        },
        dispatch(type, event) {
            for (const handler of listeners.get(type) || []) handler.listener(event);
        }
    };
}

function pointerEvent(target, button, clientX = 0, clientY = 0) {
    return {
        target,
        button,
        clientX,
        clientY,
        prevented: false,
        stopped: false,
        preventDefault() { this.prevented = true; },
        stopPropagation() { this.stopped = true; }
    };
}

function loadBridge({ clipboard, setTimeout: schedule = function() {}, clearTimeout: cancel = function() {} } = {}) {
    const document = eventTarget();
    const window = eventTarget();
    window.navigator = clipboard === undefined ? {} : { clipboard };
    document.defaultView = window;
    window.document = document;
    window.window = window;
    vm.runInNewContext(source, {
        window,
        document,
        WeakMap,
        Uint8Array,
        TextDecoder,
        Math,
        atob,
        setTimeout: schedule,
        clearTimeout: cancel,
        Promise
    });
    return { api: window.P3ClipboardBridge, document, window };
}

function terminalFixture() {
    const element = eventTarget();
    const child = eventTarget(element);
    const oscHandlers = [];
    const selectionHandlers = [];
    let oscDisposals = 0;
    let selectionDisposals = 0;
    const pasted = [];
    let selection = '';
    return {
        term: {
            element,
            parser: { registerOscHandler(code, handler) {
                const registration = { code, handler, active: true };
                oscHandlers.push(registration);
                return { dispose() { registration.active = false; oscDisposals++; } };
            } },
            onSelectionChange(handler) {
                const registration = { handler, active: true };
                selectionHandlers.push(registration);
                return { dispose() { registration.active = false; selectionDisposals++; } };
            },
            getSelection() { return selection; },
            paste(text) { pasted.push(text); }
        },
        element,
        child,
        oscHandlers,
        selectionHandlers,
        pasted,
        get oscDisposals() { return oscDisposals; },
        get selectionDisposals() { return selectionDisposals; },
        setSelection(value) {
            selection = value;
            for (const registration of selectionHandlers) {
                if (registration.active) registration.handler();
            }
        }
    };
}

function flushPromises() {
    return new Promise(resolve => setImmediate(resolve));
}

function installBridge(api, fixture, document, options = {}) {
    return api.installClipboardBridge(fixture.term, document, {
        deferSelection(callback) { return setTimeout(callback, 0); },
        cancelDeferredSelection(timer) { clearTimeout(timer); },
        ...options
    });
}
test('parses bounded OSC 52 clipboard writes', () => {
    const { api } = loadBridge();
    assert.equal(api.parseOsc52Payload('c;aGVsbG8='), 'hello');
    assert.equal(api.parseOsc52Payload(';aGVsbG8='), 'hello');
    assert.equal(api.parseOsc52Payload('c;4pyT'), '✓');
    for (const value of ['c;?', 'c;', 'missing-separator', 'c;%%%', 'p;aGVsbG8=']) {
        assert.equal(api.parseOsc52Payload(value), null);
    }
});

test('rejects oversized encoded input before decoding', () => {
    let decoded = false;
    const document = eventTarget();
    document.defaultView = { navigator: {} };
    const window = { document };
    window.window = window;
    vm.runInNewContext(source, {
        window,
        document,
        WeakMap,
        Uint8Array,
        TextDecoder,
        Math,
        atob() { decoded = true; return ''; },
        setTimeout() {}
    });
    assert.equal(window.P3ClipboardBridge.parseOsc52Payload(`c;${'A'.repeat(1398105)}`), null);
    assert.equal(decoded, false);
});

test('recognizes only Ctrl+Shift+C with an active selection', () => {
    const { api } = loadBridge();
    const base = { key: 'C', ctrlKey: true, shiftKey: true, metaKey: false, altKey: false };
    assert.equal(api.isCopyShortcut(base, true), true);
    assert.equal(api.isCopyShortcut(base, false), false);
    assert.equal(api.isCopyShortcut({ ...base, metaKey: true, ctrlKey: false }, true), false);
    assert.equal(api.isCopyShortcut({ ...base, altKey: true }, true), false);
});

test('gates OSC 52 writes on one completed tmux drag', async () => {
    const writes = [];
    const timers = [];
    const cancelled = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document, {
        dragThreshold: 4,
        osc52GestureWindowMs: 10,
        setTimeout(callback, delay) { timers.push({ callback, delay }); return timers.length; },
        clearTimeout(id) { cancelled.push(id); }
    });
    const osc52 = fixture.oscHandlers[0].handler;

    osc52('c;dW5zb2xpY2l0ZWQ=');
    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0, 10, 10));
    document.dispatch('mouseup', pointerEvent({}, 0, 10, 10));
    osc52('c;Y2xpY2s=');
    assert.deepEqual(writes, []);

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0, 10, 10));
    document.dispatch('mousemove', pointerEvent({}, 0, 14, 10));
    document.dispatch('mouseup', pointerEvent({}, 0, 13, 12));
    await flushPromises();
    assert.equal(timers.at(-1).delay, 10);
    osc52('c;ZHJhZw==');
    osc52('c;cmVwbGF5');
    await flushPromises();
    assert.deepEqual(writes, ['drag']);
    assert.deepEqual(cancelled, [1]);
});

test('consumes OSC authorization before parsing and expires deterministically', async () => {
    const writes = [];
    const timers = [];
    const deferred = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document, {
        dragThreshold: 0,
        osc52GestureWindowMs: 0,
        setTimeout(callback, delay) { timers.push({ callback, delay }); return timers.length; },
        clearTimeout() {},
        deferSelection(callback) { deferred.push(callback); return deferred.length; },
        cancelDeferredSelection() {}
    });
    const osc52 = fixture.oscHandlers[0].handler;

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    document.dispatch('mouseup', pointerEvent({}, 0));
    deferred.shift()();
    assert.equal(timers[0].delay, 0);
    osc52('malformed');
    osc52('c;c2Vjb25k');
    assert.deepEqual(writes, []);

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    document.dispatch('mouseup', pointerEvent({}, 0));
    deferred.shift()();
    timers.at(-1).callback();
    osc52('c;ZXhwaXJlZA==');
    assert.deepEqual(writes, []);
});

test('keeps one OSC write authorized after normal pointer capture release', async () => {
    const writes = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document, window } = loadBridge({ clipboard });
    window.PointerEvent = function() {};
    const fixture = terminalFixture();
    const captured = [];
    const timers = [];
    const deferred = [];
    fixture.element.setPointerCapture = pointerId => captured.push(pointerId);
    installBridge(api, fixture, document, {
        dragThreshold: 4,
        setTimeout(callback, delay) { timers.push({ callback, delay }); return timers.length; },
        clearTimeout() {},
        deferSelection(callback, delay) { deferred.push({ callback, delay }); return deferred.length; },
        cancelDeferredSelection() {}
    });

    fixture.element.dispatch('pointerdown', {
        ...pointerEvent(fixture.child, 0), pointerId: 7, pointerType: 'mouse', isPrimary: true
    });
    document.dispatch('pointermove', {
        ...pointerEvent({}, 0, 20, 20), pointerId: 8, pointerType: 'mouse', isPrimary: true
    });
    document.dispatch('pointerup', {
        ...pointerEvent({}, 0, 20, 20), pointerId: 8, pointerType: 'mouse', isPrimary: true
    });
    fixture.element.dispatch('lostpointercapture', { pointerId: 8 });
    fixture.oscHandlers[0].handler('c;bWlzbWF0Y2g=');
    assert.deepEqual(writes, []);
    assert.deepEqual(timers, []);
    document.dispatch('pointermove', {
        ...pointerEvent({}, 0, 10, 0), pointerId: 7, pointerType: 'mouse', isPrimary: true
    });
    document.dispatch('pointerup', {
        ...pointerEvent({}, 0), pointerId: 7, pointerType: 'mouse', isPrimary: true
    });
    fixture.element.dispatch('lostpointercapture', { pointerId: 7 });
    assert.equal(deferred.length, 1);
    assert.deepEqual(timers, []);
    deferred[0].callback();
    assert.equal(timers.length, 1);
    fixture.oscHandlers[0].handler('c;cG9pbnRlcg==');
    fixture.oscHandlers[0].handler('c;cmVwbGF5');
    await flushPromises();

    assert.deepEqual(captured, [7]);
    assert.deepEqual(document.listenerOptions('pointermove'), [true]);
    assert.deepEqual(document.listenerOptions('pointerup'), [true]);
    assert.deepEqual(writes, ['pointer']);
    assert.equal(timers.length, 1);

    fixture.element.dispatch('pointerdown', {
        ...pointerEvent(fixture.child, 0), pointerId: 9, pointerType: 'touch', isPrimary: true
    });
    document.dispatch('pointermove', {
        ...pointerEvent({}, 0, 20, 20), pointerId: 9, pointerType: 'touch', isPrimary: true
    });
    document.dispatch('pointerup', {
        ...pointerEvent({}, 0, 20, 20), pointerId: 9, pointerType: 'touch', isPrimary: true
    });
    fixture.oscHandlers[0].handler('c;dG91Y2g=');
    await flushPromises();
    assert.deepEqual(writes, ['pointer']);
});
test('buffers only the first OSC response until release classification', async () => {
    const writes = [];
    const deferred = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    api.installClipboardBridge(fixture.term, document, {
        dragThreshold: 0,
        deferSelection(callback) { deferred.push(callback); return deferred.length; },
        cancelDeferredSelection() {}
    });

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    document.dispatch('mouseup', pointerEvent({}, 0));
    fixture.oscHandlers[0].handler('c;ZmFzdA==');
    fixture.oscHandlers[0].handler('c;c2Vjb25k');
    assert.deepEqual(writes, []);
    deferred[0]();
    await flushPromises();

    assert.deepEqual(writes, ['fast']);
    fixture.oscHandlers[0].handler('c;cmVwbGF5');
    await flushPromises();
    assert.deepEqual(writes, ['fast']);
});


test('native selection wins over a buffered OSC response', async () => {
    const writes = [];
    const deferred = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document, {
        dragThreshold: 0,
        deferSelection(callback) { deferred.push(callback); return deferred.length; },
        cancelDeferredSelection() {}
    });

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    document.dispatch('mouseup', pointerEvent({}, 0));
    fixture.oscHandlers[0].handler('c;cmVtb3Rl');
    fixture.setSelection('native');
    deferred[0]();
    await flushPromises();

    assert.deepEqual(writes, ['native']);
});

test('classifies native selection by activity even when final text is empty', async () => {
    const writes = [];
    const timers = [];
    const deferred = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    fixture.setSelection('same');
    installBridge(api, fixture, document, {
        dragThreshold: 0,
        setTimeout(callback, delay) { timers.push({ callback, delay }); return timers.length; },
        clearTimeout() {},
        deferSelection(callback) { deferred.push(callback); return deferred.length; },
        cancelDeferredSelection() {}
    });

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    fixture.setSelection('same');
    document.dispatch('mouseup', pointerEvent({}, 0));
    deferred.shift()();
    assert.deepEqual(writes, ['same']);
    assert.deepEqual(timers, []);

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    fixture.setSelection('');
    document.dispatch('mouseup', pointerEvent({}, 0));
    deferred.shift()();
    assert.deepEqual(timers, []);
    fixture.oscHandlers[0].handler('c;dG11eA==');
    await flushPromises();
    assert.deepEqual(writes, ['same']);
});

test('copies native selection on outside release without authorizing OSC', async () => {
    const writes = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document);
    assert.deepEqual(document.listenerOptions('mousemove'), [true]);
    assert.deepEqual(document.listenerOptions('mouseup'), [true]);

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0, 1, 1));
    document.dispatch('mousemove', pointerEvent({}, 0, 20, 20));
    fixture.setSelection('native');
    document.dispatch('mouseup', pointerEvent({}, 0, 20, 20));
    fixture.oscHandlers[0].handler('c;cmVtb3Rl');
    await new Promise(resolve => setTimeout(resolve, 0));
    await flushPromises();
    assert.deepEqual(writes, ['native']);
});

test('cancels stale drag and OSC state on blur, non-left release, and new down', () => {
    const writes = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document, window } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document, { dragThreshold: 0 });
    const osc52 = fixture.oscHandlers[0].handler;
    const start = () => {
        fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
        document.dispatch('mousemove', pointerEvent({}, 0));
    };

    start(); window.dispatch('blur', {}); document.dispatch('mouseup', pointerEvent({}, 0)); osc52('c;Ymx1cg==');
    start(); document.dispatch('mouseup', pointerEvent({}, 2)); osc52('c;cmlnaHQ=');
    start(); document.dispatch('mouseup', pointerEvent({}, 0)); fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0)); osc52('c;c3RhbGU=');
    assert.deepEqual(writes, []);
});

test('leaves right-click and middle-click to the browser', async () => {
    let reads = 0;
    const clipboard = {
        readText() { reads++; return Promise.resolve('paste'); },
        writeText() { return Promise.resolve(); }
    };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    assert.equal(installBridge(api, fixture, document), true);
    assert.equal(installBridge(api, fixture, document), true);
    assert.equal(fixture.oscHandlers.length, 1);

    const right = pointerEvent(fixture.child, 2);
    document.dispatch('mousedown', right);
    document.dispatch('contextmenu', right);
    assert.equal(right.prevented, false);
    assert.equal(right.stopped, false);

    const middle = pointerEvent(fixture.child, 1);
    document.dispatch('mousedown', middle);
    document.dispatch('auxclick', middle);
    await flushPromises();
    assert.equal(middle.prevented, false);
    assert.equal(middle.stopped, false);
    assert.equal(reads, 0);
    assert.deepEqual(fixture.pasted, []);

    const outside = pointerEvent({}, 1);
    document.dispatch('mousedown', outside);
    document.dispatch('auxclick', outside);
    assert.equal(outside.prevented, false);
    assert.equal(outside.stopped, false);
    assert.equal(reads, 0);
});


test('releases captured pointers only on cancelled interactions', () => {
    const { api, document, window } = loadBridge();
    window.PointerEvent = function() {};
    const fixture = terminalFixture();
    const released = [];
    fixture.element.setPointerCapture = function() {};
    fixture.element.releasePointerCapture = pointerId => {
        released.push(pointerId);
        fixture.element.dispatch('lostpointercapture', { pointerId });
    };
    installBridge(api, fixture, document);

    fixture.element.dispatch('pointerdown', {
        ...pointerEvent(fixture.child, 0), pointerId: 11, pointerType: 'mouse', isPrimary: true
    });
    window.dispatch('blur', {});
    fixture.element.dispatch('pointerdown', {
        ...pointerEvent(fixture.child, 0), pointerId: 12, pointerType: 'mouse', isPrimary: true
    });
    fixture.element.dispatch('pointerdown', {
        ...pointerEvent(fixture.child, 0), pointerId: 13, pointerType: 'mouse', isPrimary: true
    });
    document.dispatch('pointerup', {
        ...pointerEvent({}, 0), pointerId: 13, pointerType: 'mouse', isPrimary: true
    });
    fixture.element.dispatch('pointerdown', {
        ...pointerEvent(fixture.child, 0), pointerId: 14, pointerType: 'mouse', isPrimary: true
    });
    assert.equal(api.uninstallClipboardBridge(fixture.term), true);

    assert.deepEqual(released, [11, 12, 14]);
});

test('contains synchronous clipboard write failures', () => {
    const { api, document } = loadBridge({
        clipboard: {
            writeText() { throw new Error('write denied'); }
        }
    });
    const fixture = terminalFixture();
    installBridge(api, fixture, document, { dragThreshold: 0 });

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    fixture.setSelection('native');
    assert.doesNotThrow(() => document.dispatch('mouseup', pointerEvent({}, 0)));

    const middle = pointerEvent(fixture.child, 1);
    assert.doesNotThrow(() => document.dispatch('mousedown', middle));
    assert.equal(middle.prevented, false);
    assert.equal(middle.stopped, false);
});

test('classifies a release-time selection update before deferred finalization', () => {
    const writes = [];
    const deferred = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document, {
        dragThreshold: 0,
        deferSelection(callback) { deferred.push(callback); return deferred.length; },
        cancelDeferredSelection() {}
    });

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    document.dispatch('mouseup', pointerEvent({}, 0));
    fixture.setSelection('release-time');
    deferred.shift()();
    fixture.oscHandlers[0].handler('c;dG11eA==');

    assert.deepEqual(writes, ['release-time']);
});

test('production deferral observes selection changes after mouseup', () => {
    const writes = [];
    const deferred = [];
    const clipboard = { writeText(text) { writes.push(text); return Promise.resolve(); } };
    const { api, document } = loadBridge({
        clipboard,
        setTimeout(callback) { deferred.push(callback); return deferred.length; },
        clearTimeout() {}
    });
    const fixture = terminalFixture();
    api.installClipboardBridge(fixture.term, document, { dragThreshold: 0 });

    fixture.element.dispatch('mousedown', pointerEvent(fixture.child, 0));
    document.dispatch('mousemove', pointerEvent({}, 0));
    document.dispatch('mouseup', pointerEvent({}, 0));
    fixture.setSelection('production-release-time');
    deferred.pop()();

    assert.deepEqual(writes, ['production-release-time']);
});

test('disposes idempotently and permits a clean reinstall', async () => {
    const writes = [];
    const clipboard = {
        writeText(text) { writes.push(text); return Promise.resolve(); }
    };
    const { api, document } = loadBridge({ clipboard });
    const fixture = terminalFixture();
    installBridge(api, fixture, document);
    const originalOsc = fixture.oscHandlers[0];

    assert.equal(api.uninstallClipboardBridge(fixture.term), true);
    assert.equal(api.uninstallClipboardBridge(fixture.term), false);
    assert.equal(originalOsc.active, false);
    assert.equal(fixture.oscDisposals, 1);
    assert.equal(fixture.selectionDisposals, 1);
    document.dispatch('mousedown', pointerEvent(fixture.child, 1));
    await flushPromises();
    assert.deepEqual(fixture.pasted, []);
    assert.deepEqual(writes, []);

    assert.equal(installBridge(api, fixture, document), true);
    assert.equal(fixture.oscHandlers.length, 2);
    const middle = pointerEvent(fixture.child, 1);
    document.dispatch('mousedown', middle);
    document.dispatch('auxclick', middle);
    await flushPromises();
    assert.deepEqual(fixture.pasted, []);
    assert.equal(middle.prevented, false);
    assert.equal(middle.stopped, false);
});

test('waits until ttyd exposes an opened terminal element', () => {
    const callbacks = [];
    const { api, window } = loadBridge({ setTimeout: callback => callbacks.push(callback) });
    const fixture = terminalFixture();
    window.term = { parser: fixture.term.parser };
    assert.equal(api.installWhenReady(window, 1), false);
    assert.equal(fixture.oscHandlers.length, 0);
    window.term = fixture.term;
    callbacks.shift()();
    assert.equal(fixture.oscHandlers.length, 1);
});

test('reports when ttyd never exposes a terminal', () => {
    const warnings = [];
    const { api, window } = loadBridge();
    window.console = { warn(message) { warnings.push(message); } };
    assert.equal(api.installWhenReady(window, 0), false);
    assert.deepEqual(warnings, ['P3 clipboard bridge could not find the ttyd terminal']);
});
