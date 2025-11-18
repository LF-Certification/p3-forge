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

    // Initialize Split.js for resizable panes (only if instructions exist and desktop)
    let splitInstance = null;
    let isCollapsed = false;

    if (instructionsTool && window.innerWidth >= 768) {
        splitInstance = Split(["#instructions-sidebar", "#tool-pane"], {
            sizes: [25, 75],
            minSize: [0, 400],
            gutterSize: 10,
            cursor: 'col-resize',
            snapOffset: 0
        });

        // Create expand tab for collapsed state
        const expandTab = document.createElement('div');
        expandTab.className = 'instructions-expand-tab';
        // Wrap letters in individual divs, append a matching chevron icon
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
        expandTab.setAttribute('aria-label', 'Expand instructions');
        document.body.appendChild(expandTab);

        // Create toggle button on gutter
        const gutter = document.querySelector('.gutter');
        if (gutter) {
            const toggleBtn = document.createElement('button');
            toggleBtn.className = 'gutter-toggle';
            toggleBtn.setAttribute('aria-label', 'Toggle instructions panel');
            toggleBtn.innerHTML = `<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
                <path fill-rule="evenodd" d="M11.354 1.646a.5.5 0 0 1 0 .708L5.707 8l5.647 5.646a.5.5 0 0 1-.708.708l-6-6a.5.5 0 0 1 0-.708l6-6a.5.5 0 0 1 .708 0z"/>
            </svg>`;
            const toggleIcon = toggleBtn.querySelector('svg');
            gutter.appendChild(toggleBtn);

            // Toggle function
            function toggleInstructions() {
                if (isCollapsed) {
                    // Expand
                    splitInstance.setSizes([25, 75]);
                    isCollapsed = false;
                    expandTab.classList.remove('visible');
                    toggleIcon.classList.remove('flipped');
                    toggleBtn.setAttribute('aria-label', 'Collapse instructions panel');
                } else {
                    // Collapse
                    splitInstance.setSizes([0, 100]);
                    isCollapsed = true;
                    expandTab.classList.add('visible');
                    toggleIcon.classList.add('flipped');
                    toggleBtn.setAttribute('aria-label', 'Expand instructions panel');
                }
            }

            // Attach event listeners
            toggleBtn.addEventListener('click', toggleInstructions);
            expandTab.addEventListener('click', toggleInstructions);
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

    function updateTimer() {
        const now = new Date();
        const timeLeft = expirationDate - now;

        if (timeLeft <= 0) {
            document.getElementById('timer-display').textContent = '00:00';
            // Add a small delay to ensure the user sees 00:00 briefly
            setTimeout(handleExpiration, 1000);
            return;
        }

        const minutes = Math.floor(timeLeft / (1000 * 60));
        const seconds = Math.floor((timeLeft % (1000 * 60)) / 1000);

        const formattedTime = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        document.getElementById('timer-display').textContent = formattedTime;
    }

    if (showTimer) {
        updateTimer();
        setInterval(updateTimer, 1000);
    }
});
