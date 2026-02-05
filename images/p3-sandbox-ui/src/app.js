document.addEventListener('DOMContentLoaded', function() {
    let config;
    let tools;
    let defaultTool;
    let expirationDate;
    let hasExpired = false;
    let showTimer = true;

    try {
        const parsedConfig = JSON.parse(configStr);
        config = parsedConfig.config;
        tools = parsedConfig.tools;
        defaultTool = config.defaultTool;
        expirationDate = new Date(config.expiresAt);
        showTimer = config.showTimer !== false;
    } catch (error) {
        console.error('Error parsing configuration:', error);
        return;
    }

    const countdownTimer = document.getElementById('countdown-timer');
    if (countdownTimer) {
        countdownTimer.style.display = showTimer ? '' : 'none';
    }

    // Separate instructions tool from other tools
    const instructionsTool = tools.find(tool => tool.kind === 'instructions');
    const otherTools = tools.filter(tool => tool.kind !== 'instructions');

    // Set up instructions sidebar - hide if no instructions tool
    const instructionsSidebar = document.querySelector('#instructions-sidebar');
    const toolPane = document.querySelector('#tool-pane');

    if (instructionsTool) {
        const instructionsPane = document.querySelector('#instructions-pane');
        if (instructionsPane) {
            instructionsPane.innerHTML = `
                <iframe src="${instructionsTool.url}"
                        title="${instructionsTool.name}"
                        class="w-100 border-0 h-100"
                        loading="lazy"
                        sandbox="allow-same-origin allow-scripts allow-popups allow-forms">
                </iframe>
            `;
        }
    } else {
        // Hide sidebar and expand tool pane to full width
        if (instructionsSidebar) {
            instructionsSidebar.style.display = 'none';
        }
        if (toolPane) {
            toolPane.style.flex = '1 1 100%';
            toolPane.style.width = '100%';
        }
    }

    // Initialize Split.js for resizable panes (instructions + desktop only)
    const DESKTOP_BREAKPOINT = 768;
    const SPLIT_STATE_STORAGE_KEY = 'lf-sandbox-ui-split-state-v1';
    const DEFAULT_SPLIT_SIZES = [25, 75];
    let splitInstance = null;
    let isCollapsed = false;
    let savedSplitSizes = [...DEFAULT_SPLIT_SIZES];
    let resizeDebounceTimer = null;
    let expandTab = null;
    let toggleBtn = null;
    let toggleIcon = null;
    let desktopMediaQuery = null;
    let rootResizeObserver = null;

    function sanitizeSplitSizes(value) {
        if (!Array.isArray(value) || value.length !== 2) {
            return [...DEFAULT_SPLIT_SIZES];
        }

        const left = Number(value[0]);
        const right = Number(value[1]);
        if (!Number.isFinite(left) || !Number.isFinite(right)) {
            return [...DEFAULT_SPLIT_SIZES];
        }

        const total = left + right;
        if (total <= 0) {
            return [...DEFAULT_SPLIT_SIZES];
        }

        const normalizedLeft = Math.max(0, Math.min(100, (left / total) * 100));
        const normalizedRight = 100 - normalizedLeft;
        return [normalizedLeft, normalizedRight];
    }

    function readPersistedSplitState() {
        try {
            const stored = sessionStorage.getItem(SPLIT_STATE_STORAGE_KEY);
            if (!stored) {
                return;
            }

            const parsed = JSON.parse(stored);
            savedSplitSizes = sanitizeSplitSizes(parsed.sizes);
            isCollapsed = parsed.collapsed === true;
        } catch (error) {
            // Ignore malformed session state and fall back to defaults.
        }
    }

    function persistSplitState() {
        if (!instructionsTool) {
            return;
        }

        try {
            const payload = JSON.stringify({
                sizes: savedSplitSizes,
                collapsed: isCollapsed
            });
            sessionStorage.setItem(SPLIT_STATE_STORAGE_KEY, payload);
        } catch (error) {
            // Ignore storage write failures (e.g., blocked storage policies).
        }
    }

    function clearSplitInlineSizing() {
        [instructionsSidebar, toolPane].forEach((pane) => {
            if (!pane) {
                return;
            }
            pane.style.removeProperty('width');
            pane.style.removeProperty('min-width');
            pane.style.removeProperty('max-width');
            pane.style.removeProperty('flex');
            pane.style.removeProperty('flex-basis');
            pane.style.removeProperty('flex-grow');
            pane.style.removeProperty('flex-shrink');
        });
    }

    function updateSplitToggleUI() {
        const shouldShowExpandControl = Boolean(splitInstance && isCollapsed);

        if (expandTab) {
            expandTab.classList.toggle('visible', shouldShowExpandControl);
            expandTab.setAttribute('aria-hidden', shouldShowExpandControl ? 'false' : 'true');
            expandTab.setAttribute('aria-label', 'Expand instructions panel');
            expandTab.setAttribute('tabindex', shouldShowExpandControl ? '0' : '-1');
        }

        if (toggleBtn) {
            toggleBtn.setAttribute(
                'aria-label',
                isCollapsed ? 'Expand instructions panel' : 'Collapse instructions panel'
            );
        }

        if (toggleIcon) {
            toggleIcon.classList.toggle('flipped', isCollapsed);
        }
    }

    function toggleInstructions() {
        if (!splitInstance) {
            return;
        }

        if (isCollapsed) {
            splitInstance.setSizes(savedSplitSizes);
            isCollapsed = false;
        } else {
            const currentSizes = typeof splitInstance.getSizes === 'function'
                ? splitInstance.getSizes()
                : null;

            if (Array.isArray(currentSizes) && currentSizes[0] > 0) {
                savedSplitSizes = sanitizeSplitSizes(currentSizes);
            }

            splitInstance.setSizes([0, 100]);
            isCollapsed = true;
        }

        updateSplitToggleUI();
        persistSplitState();
    }

    function handleExpandTabKeydown(event) {
        if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            toggleInstructions();
        }
    }

    function ensureExpandTab() {
        if (expandTab) {
            return;
        }

        expandTab = document.createElement('div');
        expandTab.className = 'instructions-expand-tab';

        // Wrap letters individually for vertical label styling.
        const instructionsHtml = 'Instructions'.split('').map(char =>
            char === ' ' ? '<span style="height: 8px"></span>' : `<div>${char}</div>`
        ).join('');

        expandTab.innerHTML = `
            <div class="instructions-expand-label">${instructionsHtml}</div>
            <div class="instructions-expand-arrow">
                <svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg" fill="currentColor" class="flipped">
                    <path fill-rule="evenodd" d="M11.354 1.646a.5.5 0 0 1 0 .708L5.707 8l5.647 5.646a.5.5 0 0 1-.708.708l-6-6a.5.5 0 0 1 0-.708l6-6a.5.5 0 0 1 .708 0z"/>
                </svg>
            </div>
        `;

        expandTab.setAttribute('role', 'button');
        expandTab.setAttribute('aria-hidden', 'true');
        expandTab.setAttribute('tabindex', '-1');
        expandTab.addEventListener('click', toggleInstructions);
        expandTab.addEventListener('keydown', handleExpandTabKeydown);
        document.body.appendChild(expandTab);
    }

    function attachGutterToggle() {
        const gutter = document.querySelector('.gutter');
        if (!gutter) {
            return;
        }

        if (toggleBtn) {
            toggleBtn.remove();
            toggleBtn = null;
            toggleIcon = null;
        }

        toggleBtn = document.createElement('button');
        toggleBtn.className = 'gutter-toggle';
        toggleBtn.innerHTML = `<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
            <path fill-rule="evenodd" d="M11.354 1.646a.5.5 0 0 1 0 .708L5.707 8l5.647 5.646a.5.5 0 0 1-.708.708l-6-6a.5.5 0 0 1 0-.708l6-6a.5.5 0 0 1 .708 0z"/>
        </svg>`;

        toggleIcon = toggleBtn.querySelector('svg');
        toggleBtn.addEventListener('click', toggleInstructions);
        gutter.appendChild(toggleBtn);
        updateSplitToggleUI();
    }

    function destroySplitLayout() {
        if (splitInstance) {
            splitInstance.destroy();
            splitInstance = null;
        }

        if (toggleBtn) {
            toggleBtn.remove();
            toggleBtn = null;
            toggleIcon = null;
        }

        if (expandTab) {
            expandTab.classList.remove('visible');
            expandTab.setAttribute('aria-hidden', 'true');
            expandTab.setAttribute('tabindex', '-1');
        }

        clearSplitInlineSizing();
    }

    function initDesktopSplitLayout() {
        if (splitInstance || !instructionsTool) {
            return;
        }

        splitInstance = Split(["#instructions-sidebar", "#tool-pane"], {
            sizes: isCollapsed ? [0, 100] : savedSplitSizes,
            minSize: [0, 400],
            gutterSize: 10,
            cursor: 'col-resize',
            snapOffset: 0,
            onDragEnd: () => {
                if (isCollapsed || typeof splitInstance.getSizes !== 'function') {
                    return;
                }

                const sizes = splitInstance.getSizes();
                if (Array.isArray(sizes) && sizes[0] > 0) {
                    savedSplitSizes = sanitizeSplitSizes(sizes);
                    persistSplitState();
                }
            }
        });

        ensureExpandTab();
        attachGutterToggle();
        updateSplitToggleUI();
    }

    function updateResponsiveSplitLayout() {
        if (!instructionsTool) {
            return;
        }

        if (window.innerWidth >= DESKTOP_BREAKPOINT) {
            initDesktopSplitLayout();
        } else {
            destroySplitLayout();
        }
    }

    function scheduleResponsiveSplitUpdate() {
        window.clearTimeout(resizeDebounceTimer);
        resizeDebounceTimer = window.setTimeout(updateResponsiveSplitLayout, 120);
    }

    if (instructionsTool) {
        readPersistedSplitState();
        updateResponsiveSplitLayout();

        window.addEventListener('resize', scheduleResponsiveSplitUpdate);
        window.addEventListener('orientationchange', scheduleResponsiveSplitUpdate);

        if (window.visualViewport) {
            window.visualViewport.addEventListener('resize', scheduleResponsiveSplitUpdate);
        }

        desktopMediaQuery = window.matchMedia(`(min-width: ${DESKTOP_BREAKPOINT}px)`);
        if (typeof desktopMediaQuery.addEventListener === 'function') {
            desktopMediaQuery.addEventListener('change', scheduleResponsiveSplitUpdate);
        } else if (typeof desktopMediaQuery.addListener === 'function') {
            desktopMediaQuery.addListener(scheduleResponsiveSplitUpdate);
        }

        if (typeof ResizeObserver === 'function') {
            rootResizeObserver = new ResizeObserver(scheduleResponsiveSplitUpdate);
            rootResizeObserver.observe(document.documentElement);
        }
    }

    // Set up other tools in tabs
    const navPills = document.querySelector('.nav.nav-pills');
    const tabContent = document.querySelector('.tab-content');

    navPills.innerHTML = '';
    tabContent.innerHTML = '';

    otherTools.forEach((tool, index) => {
        const isDefault = tool.name === defaultTool;

        const navItem = document.createElement('li');
        navItem.className = 'nav-item';
        navItem.innerHTML = `
            <a class="nav-link ${isDefault ? 'active' : ''}"
               href="#"
               data-bs-toggle="tab"
               data-bs-target="#${tool.name}"
               data-tool-url="${tool.url}">
                ${tool.name}
            </a>
        `;
        navPills.appendChild(navItem);

        const tabPane = document.createElement('div');
        tabPane.className = `tab-pane ${isDefault ? 'active show' : ''} h-100 overflow-hidden`;
        tabPane.id = tool.name;
        tabPane.innerHTML = `
            <iframe src="${isDefault ? tool.url : 'about:blank'}"
                    title="${tool.name}"
                    class="w-100 border-0 h-100"
                    loading="lazy"
                    sandbox="allow-same-origin allow-scripts allow-popups allow-forms">
            </iframe>
        `;
        tabContent.appendChild(tabPane);
    });

    document.querySelectorAll('.nav-link').forEach(link => {
        link.addEventListener('shown.bs.tab', function(event) {
            const targetId = event.target.getAttribute('data-bs-target').substring(1);
            const toolUrl = event.target.getAttribute('data-tool-url');
            const targetPane = document.querySelector(`#${targetId}`);
            const targetIframe = targetPane.querySelector('iframe');

            if (targetIframe.src === 'about:blank' || targetIframe.src === window.location.href + 'about:blank') {
                targetIframe.src = toolUrl;
            }
        });
    });

    function handleExpiration() {
        if (!hasExpired) {
            hasExpired = true;
            // Redirect to the expired page
            window.location.href = '/expired.html';
        }
    }

    function formatTimeLeft(timeLeftMs) {
        const totalSeconds = Math.max(0, Math.floor(timeLeftMs / 1000));
        const totalHours = Math.floor(totalSeconds / 3600);
        const minutes = Math.floor((totalSeconds % 3600) / 60);
        const seconds = totalSeconds % 60;
        return `${totalHours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    }

    function updateTimer() {
        const now = new Date();
        const timeLeft = expirationDate - now;

        if (timeLeft <= 0) {
            document.getElementById('timer-display').textContent = '00:00:00';
            // Add a small delay to ensure the user sees 00:00 briefly
            setTimeout(handleExpiration, 1000);
            return;
        }

        const formattedTime = formatTimeLeft(timeLeft);
        document.getElementById('timer-display').textContent = formattedTime;
    }

    if (showTimer) {
        updateTimer();
        setInterval(updateTimer, 1000);
    }
});
