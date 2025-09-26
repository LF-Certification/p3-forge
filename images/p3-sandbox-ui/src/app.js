document.addEventListener('DOMContentLoaded', function() {
    let config;
    let tools;
    let defaultTool;
    let expirationDate;
    let hasExpired = false;

    try {
        const parsedConfig = JSON.parse(configStr);
        config = parsedConfig.config;
        tools = parsedConfig.tools;
        defaultTool = config.defaultTool;
        expirationDate = new Date(config.expiresAt);
    } catch (error) {
        console.error('Error parsing configuration:', error);
        return;
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
            toolPane.classList.remove('col-md-9');
            toolPane.classList.add('col-12');
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

    updateTimer();
    setInterval(updateTimer, 1000);
});
