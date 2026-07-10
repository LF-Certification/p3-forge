(function() {
    'use strict';

    // Maximum decoded OSC 52 payload we accept (tmux caps its own at ~100KB).
    const MAX_OSC52_BYTES = 1024 * 1024;
    const MAX_OSC52_BASE64_CHARS = Math.ceil(MAX_OSC52_BYTES / 3) * 4;
    const DRAG_THRESHOLD_PX = 4;
    const OSC52_GESTURE_WINDOW_MS = 250;
    const terminalInstallations = new WeakMap();

    function decodeBase64Utf8(base64) {
        if (base64.length > MAX_OSC52_BASE64_CHARS) {
            return null;
        }
        const binary = atob(base64);
        if (binary.length > MAX_OSC52_BYTES) {
            return null;
        }
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return new TextDecoder().decode(bytes);
    }

    // Parses an OSC 52 payload ("Pc;Pd" after the "52;" prefix) and returns
    // the decoded clipboard text, or null when the payload is not a
    // set-clipboard request we should act on.
    function parseOsc52Payload(data) {
        if (typeof data !== 'string') {
            return null;
        }
        const separator = data.indexOf(';');
        if (separator === -1) {
            return null;
        }
        const target = data.slice(0, separator);
        if (target !== '' && target !== 'c') {
            return null;
        }
        const payload = data.slice(separator + 1);
        // "?" is a clipboard-read query; never answer it (information leak).
        if (payload === '?' || payload === '') {
            return null;
        }
        try {
            return decodeBase64Utf8(payload);
        } catch (e) {
            return null;
        }
    }

    function getClipboard(doc) {
        const view = doc && doc.defaultView;
        const nav = view && view.navigator;
        return nav && nav.clipboard;
    }

    function writeClipboard(doc, text) {
        const clipboard = getClipboard(doc);
        if (!text || !clipboard || typeof clipboard.writeText !== 'function') {
            return;
        }
        try {
            Promise.resolve(clipboard.writeText(text)).catch(function() {
                // Permission denied: degrade silently to manual copy.
            });
        } catch (e) {
            // Nonconforming webviews may throw before returning a promise.
        }
    }


    // Ctrl+Shift+C copies the active selection (VS Code / GNOME Terminal
    // muscle memory). Deliberately not Cmd+Shift+C: Safari opens devtools
    // on it and page JS cannot suppress that; macOS users have plain
    // Cmd+C for xterm selections. Without a selection the key passes
    // through to the pty unchanged.
    function isCopyShortcut(event, hasSelection) {
        if (!event || !hasSelection) {
            return false;
        }
        const key = typeof event.key === 'string' ? event.key.toLowerCase() : '';
        return key === 'c' && event.shiftKey && event.ctrlKey && !event.metaKey && !event.altKey;
    }

    function installClipboardBridge(term, doc, options) {
        if (terminalInstallations.has(term)) {
            return true;
        }

        const element = term.element;
        if (!element || !term.parser || typeof element.addEventListener !== 'function' ||
                typeof term.parser.registerOscHandler !== 'function' || typeof term.onSelectionChange !== 'function') {
            return false;
        }

        const settings = options || {};
        const schedule = typeof settings.setTimeout === 'function' ? settings.setTimeout : setTimeout;
        const cancelSchedule = typeof settings.clearTimeout === 'function' ? settings.clearTimeout : clearTimeout;
        const deferSelection = typeof settings.deferSelection === 'function' ? settings.deferSelection : setTimeout;
        const cancelDeferredSelection = typeof settings.cancelDeferredSelection === 'function' ?
            settings.cancelDeferredSelection : clearTimeout;
        const gestureWindowMs = settings.osc52GestureWindowMs ?? OSC52_GESTURE_WINDOW_MS;
        const dragThreshold = settings.dragThreshold ?? DRAG_THRESHOLD_PX;
        const view = doc.defaultView;
        const supportsPointerEvents = view && typeof view.PointerEvent === 'function';
        const downEvent = supportsPointerEvents ? 'pointerdown' : 'mousedown';
        const moveEvent = supportsPointerEvents ? 'pointermove' : 'mousemove';
        const upEvent = supportsPointerEvents ? 'pointerup' : 'mouseup';
        let selecting = false;
        let moved = false;
        let selectionChanged = false;
        let startX = 0;
        let startY = 0;
        let activePointerId = null;
        let osc52Authorized = false;
        let osc52Timer = null;
        let selectionTimer = null;
        let disposed = false;
        let pendingRelease = null;

        function targetsTerminal(event) {
            return event.target === element || element.contains(event.target);
        }

        function clearOsc52Authorization() {
            osc52Authorized = false;
            if (osc52Timer !== null) {
                cancelSchedule(osc52Timer);
                osc52Timer = null;
            }
        }
        function clearSelectionTimer() {
            if (selectionTimer !== null) {
                cancelDeferredSelection(selectionTimer);
                selectionTimer = null;
            }
            pendingRelease = null;
        }
        function clearDragState() {
            selecting = false;
            moved = false;
            selectionChanged = false;
            activePointerId = null;
        }
        function cancelInteraction() {
            const pointerId = selecting ? activePointerId : null;
            clearDragState();
            if (supportsPointerEvents && pointerId !== null && typeof element.releasePointerCapture === 'function') {
                try {
                    element.releasePointerCapture(pointerId);
                } catch (e) {
                    // Capture may already have been released by the browser.
                }
            }
            clearOsc52Authorization();
            clearSelectionTimer();
        }
        function authorizeOneOsc52Write() {
            clearOsc52Authorization();
            osc52Authorized = true;
            osc52Timer = schedule(clearOsc52Authorization, gestureWindowMs);
        }
        function handleOsc52(data) {
            if (pendingRelease) {
                if (pendingRelease.completedDrag && pendingRelease.osc52Data === null) {
                    pendingRelease.osc52Data = data;
                }
                return true;
            }
            if (!osc52Authorized) {
                return true;
            }
            clearOsc52Authorization();
            writeClipboard(doc, parseOsc52Payload(data));
            return true;
        }
        function handleDown(event) {
            cancelInteraction();
            if (event.button !== 0 || (supportsPointerEvents && (event.isPrimary === false ||
                    (event.pointerType && event.pointerType !== 'mouse' && event.pointerType !== 'pen')))) {
                return;
            }
            selecting = true;
            activePointerId = supportsPointerEvents ? event.pointerId : null;
            startX = event.clientX || 0;
            startY = event.clientY || 0;
            if (supportsPointerEvents && typeof element.setPointerCapture === 'function') {
                try {
                    element.setPointerCapture(activePointerId);
                } catch (e) {
                    clearDragState();
                }
            }
        }
        function handleMove(event) {
            if (!selecting || moved || (supportsPointerEvents && event.pointerId !== activePointerId)) {
                return;
            }
            const deltaX = (event.clientX || 0) - startX;
            const deltaY = (event.clientY || 0) - startY;
            moved = deltaX * deltaX + deltaY * deltaY >= dragThreshold * dragThreshold;
        }
        function handleSelectionChange() {
            if (selecting) {
                selectionChanged = true;
            } else if (pendingRelease) {
                pendingRelease.nativeSelection = true;
            }
        }
        function handleUp(event) {
            if (supportsPointerEvents && selecting && event.pointerId !== activePointerId) {
                return;
            }
            if (!selecting) {
                clearOsc52Authorization();
                return;
            }
            if (event.button !== 0) {
                cancelInteraction();
                return;
            }
            const release = {
                completedDrag: moved,
                nativeSelection: selectionChanged,
                osc52Data: null
            };
            clearDragState();
            clearSelectionTimer();
            pendingRelease = release;
            selectionTimer = deferSelection(function() {
                selectionTimer = null;
                if (disposed || pendingRelease !== release) {
                    return;
                }
                pendingRelease = null;
                if (!release.completedDrag) {
                    return;
                }
                const completedSelection = typeof term.getSelection === 'function' ? term.getSelection() : '';
                if (release.nativeSelection) {
                    writeClipboard(doc, completedSelection);
                } else if (release.osc52Data !== null) {
                    writeClipboard(doc, parseOsc52Payload(release.osc52Data));
                } else {
                    authorizeOneOsc52Write();
                }
            }, 0);
        }
        function handleLostPointerCapture(event) {
            if (selecting && event.pointerId === activePointerId) {
                cancelInteraction();
            }
        }
        function handleKeyDown(event) {
            const selection = typeof term.getSelection === 'function' ? term.getSelection() : '';
            if (!isCopyShortcut(event, selection !== '')) {
                return;
            }
            event.preventDefault();
            event.stopPropagation();
            writeClipboard(doc, selection);
        }

        const oscDisposable = term.parser.registerOscHandler(52, handleOsc52);
        const selectionDisposable = term.onSelectionChange(handleSelectionChange);
        element.addEventListener(downEvent, handleDown);
        doc.addEventListener(moveEvent, handleMove, true);
        doc.addEventListener(upEvent, handleUp, true);
        if (supportsPointerEvents) {
            element.addEventListener('lostpointercapture', handleLostPointerCapture);
        }
        if (view && typeof view.addEventListener === 'function') {
            view.addEventListener('blur', cancelInteraction);
        }
        doc.addEventListener('keydown', handleKeyDown, true);

        function dispose() {
            if (disposed) {
                return;
            }
            disposed = true;
            cancelInteraction();
            element.removeEventListener(downEvent, handleDown);
            doc.removeEventListener(moveEvent, handleMove, true);
            doc.removeEventListener(upEvent, handleUp, true);
            if (supportsPointerEvents) {
                element.removeEventListener('lostpointercapture', handleLostPointerCapture);
            }
            if (view && typeof view.removeEventListener === 'function') {
                view.removeEventListener('blur', cancelInteraction);
            }
            doc.removeEventListener('keydown', handleKeyDown, true);
            if (oscDisposable && typeof oscDisposable.dispose === 'function') {
                oscDisposable.dispose();
            }
            if (selectionDisposable && typeof selectionDisposable.dispose === 'function') {
                selectionDisposable.dispose();
            }
            terminalInstallations.delete(term);
        }

        terminalInstallations.set(term, dispose);
        return true;
    }

    function uninstallClipboardBridge(term) {
        const dispose = terminalInstallations.get(term);
        if (!dispose) {
            return false;
        }
        dispose();
        return true;
    }

    // ttyd assigns window.term asynchronously. Retry for a bounded period and
    // emit a diagnostic instead of failing silently when initialization stalls.
    function installWhenReady(win, attempts) {
        const remaining = typeof attempts === 'number' ? attempts : 200;
        if (win.term && win.term.parser && win.term.element && installClipboardBridge(win.term, win.document)) {
            return true;
        }
        if (remaining <= 0) {
            const logger = win.console && typeof win.console.warn === 'function' ? win.console : null;
            if (logger) {
                logger.warn('P3 clipboard bridge could not find the ttyd terminal');
            }
            return false;
        }
        setTimeout(function() {
            installWhenReady(win, remaining - 1);
        }, 50);
        return false;
    }

    Object.defineProperty(window, 'P3ClipboardBridge', {
        configurable: true,
        value: {
            parseOsc52Payload,
            isCopyShortcut,
            installClipboardBridge,
            uninstallClipboardBridge,
            installWhenReady
        }
    });
    installWhenReady(window);
})();
