(function() {
    'use strict';

    const CANCELLED_CTRL_KEYS = new Set(['f', 'p', 's', 'u']);

    function isEditableTarget(target) {
        if (!target || target === window || target === document) {
            return false;
        }

        if (target.isContentEditable) {
            return true;
        }

        const tagName = typeof target.tagName === 'string' ? target.tagName.toLowerCase() : '';
        if (tagName === 'textarea') {
            return true;
        }

        if (tagName !== 'input') {
            return false;
        }

        const type = (target.getAttribute('type') || 'text').toLowerCase();
        return !['button', 'checkbox', 'color', 'file', 'hidden', 'image', 'radio', 'range', 'reset', 'submit'].includes(type);
    }

    function shouldCancelBrowserShortcut(event) {
        if (!event || event.defaultPrevented) {
            return false;
        }

        const key = typeof event.key === 'string' ? event.key.toLowerCase() : '';
        if ((event.ctrlKey || event.metaKey) && !event.altKey && CANCELLED_CTRL_KEYS.has(key)) {
            return true;
        }

        if (key === 'f1') {
            return true;
        }

        return key === 'backspace' && !isEditableTarget(event.target);
    }

    function installBrowserShortcutGuard(target) {
        const eventTarget = target || window;
        eventTarget.addEventListener('keydown', function(event) {
            if (shouldCancelBrowserShortcut(event)) {
                event.preventDefault();
            }
        }, true);
    }

    window.P3BrowserShortcutGuard = {
        shouldCancelBrowserShortcut,
        installBrowserShortcutGuard
    };

    installBrowserShortcutGuard(window);
})();
