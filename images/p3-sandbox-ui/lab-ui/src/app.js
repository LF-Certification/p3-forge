/**
 * Lab UI Application
 *
 * This module handles the setup of a web-based UI for accessing various tools
 * in an exam environment. It fetches configuration, sets up the instructions panel,
 * and dynamically creates tabs for each configured tool.
 */

// Main initialization function
async function initializeLabUI() {
  try {
    // Fetch configuration
    const config = await fetchConfiguration();

    // Set up the instructions panel
    setupInstructionsPanel(config);

    // Set up tool tabs
    setupToolTabs(config);

    // Start the countdown timer
    startCountdownTimer();

    // Start polling for configuration changes
    startConfigPolling();
  } catch (error) {
    console.error("Error initializing UI:", error);
  }
}

/**
 * Fetches the configuration from inline variable or falls back to server
 * @returns {Promise<Object>} The configuration object
 */
async function fetchConfiguration() {
  // First try to get configuration from inline variable
  if (typeof configStr !== 'undefined' && configStr !== 'UI_CONFIG_PLACEHOLDER') {
    try {
      return JSON.parse(configStr);
    } catch (error) {
      console.error("Error parsing inline configuration:", error);
    }
  }

  // Fall back to fetching from server (for backward compatibility)
  try {
    const response = await fetch(
      new URL("config.json", window.location.href).href,
    );
    if (!response.ok) {
      throw new Error(
        `Failed to fetch config: ${response.status} ${response.statusText}`,
      );
    }
    return response.json();
  } catch (error) {
    throw new Error(`Configuration not available: ${error.message}`);
  }
}

/**
 * Sets up the instructions panel based on configuration
 * @param {Object} config - The configuration object
 */
function setupInstructionsPanel(config) {
  const instructions = document.getElementById("instructions");
  const rightPanel = document.querySelector(".col-9");
  const instructionsButton = document.getElementById("instructions-button");
  const instructionsContent = instructions.querySelector(".markdown-body");

  // Don't set content as it's already injected during build
  // Only hide instructions if they're empty (just whitespace or no content)
  const isEmpty = !instructionsContent.innerHTML.trim();

  if (isEmpty) {
    instructionsButton.classList.add("d-none");
    instructions.classList.remove("show");
    instructions.classList.add("collapse");
    rightPanel.classList.remove("col-9");
    rightPanel.classList.add("col-12");
  }

  // Handle collapse events
  instructions.addEventListener("hidden.bs.collapse", () => {
    rightPanel.classList.remove("col-9");
    rightPanel.classList.add("col-12");
    instructionsButton.classList.remove("active");
  });

  instructions.addEventListener("shown.bs.collapse", () => {
    rightPanel.classList.remove("col-12");
    rightPanel.classList.add("col-9");
    instructionsButton.classList.add("active");
  });
}

/**
 * Sets up tool tabs based on configuration
 * @param {Object} config - The configuration object
 */
function setupToolTabs(config) {
  const tabTemplate = document.getElementById("tool-tab");
  const contentTemplate = document.getElementById("tool");
  const tabContent = document.querySelector(".tab-content");

  // Validate configuration
  if (!config.tools || !Array.isArray(config.tools)) {
    throw new Error("Config must have a tools array");
  }

  // Find the default tool (if any)
  const defaultToolIndex = config.tools.findIndex(
    (tool) => tool.default === true,
  );

  // If we have a default tool, reorder the array to put it first
  if (defaultToolIndex > 0) {
    const defaultTool = config.tools[defaultToolIndex];
    config.tools.splice(defaultToolIndex, 1);
    config.tools.unshift(defaultTool);
  }

  // Store loaded iframes to prevent reloading
  const loadedIframes = new Map();

  // Create all content panes first
  config.tools.forEach((item, index) => {
    if (!item.id || !item.title) {
      throw new Error("Config items must have id and title");
    }

    // Convert relative URLs to absolute
    item.url = item.url.startsWith("/")
      ? new URL(item.url.slice(1), window.location.href).href
      : item.url;

    // Create content pane
    const newContent = createToolContent(contentTemplate, item, index);
    const iframe = newContent.querySelector("iframe");

    // Add iframe to tracking map
    loadedIframes.set(item.id, { iframe, loaded: false, url: item.url });

    // Load all iframes with blank page initially - they will be populated on demand
    iframe.src = "about:blank";

    // Prevent beforeunload events from bubbling up
    iframe.addEventListener("load", () => {
      // Only handle when loading actual content, not about:blank
      if (iframe.src === "about:blank") return;

      try {
        // Add event listener to prevent beforeunload events
        if (iframe.contentWindow) {
          // Suppress beforeunload events
          iframe.contentWindow.addEventListener(
            "beforeunload",
            (event) => {
              // Prevent the event from propagating
              event.stopImmediatePropagation();
              event.stopPropagation();
              event.preventDefault();
              // Clear the returnValue to prevent dialog
              event.returnValue = "";
              return "";
            },
            true,
          );

          // Mark as loaded
          loadedIframes.get(item.id).loaded = true;
        }
      } catch (e) {
        // Ignore cross-origin errors
        console.warn("Could not add event listener to iframe:", e);
      }
    });
  });

  // Now create tabs and link them to the content panes
  config.tools.forEach((item, index) => {
    // Create tab
    const newTab = createToolTab(tabTemplate, item, index);
    const tabLink = newTab.children[0];

    // Handle tab switching with lazy loading
    tabLink.addEventListener("shown.bs.tab", () => {
      const iframeData = loadedIframes.get(item.id);
      if (!iframeData.loaded) {
        // Set src only if not loaded yet
        iframeData.iframe.src = iframeData.url;
      }
    });
  });

  // Load the first iframe immediately
  if (config.tools.length > 0) {
    const firstItem = config.tools[0];
    const firstIframe = loadedIframes.get(firstItem.id).iframe;
    firstIframe.src = loadedIframes.get(firstItem.id).url;
  }

  // Remove templates after all tabs are created
  tabTemplate.remove();
  contentTemplate.remove();

  // Prevent any accidental beforeunload for the main window
  window.addEventListener(
    "beforeunload",
    (event) => {
      const activeTabContent = document.querySelector(".tab-pane.active");
      if (activeTabContent) {
        // Prevent the event if it's coming from an iframe
        event.stopPropagation();
        event.preventDefault();
        event.returnValue = "";
        return "";
      }
    },
    true,
  );
}

/**
 * Creates a tab element for a tool
 * @param {HTMLElement} template - The tab template element
 * @param {Object} item - The tool configuration
 * @param {number} index - The index of the tool
 * @returns {HTMLElement} The created tab element
 */
function createToolTab(template, item, index) {
  const newTab = template.cloneNode(true);
  newTab.id = `${item.id}-tab`;

  const tabLink = newTab.children[0];
  tabLink.setAttribute("href", `#${item.id}`);
  tabLink.setAttribute("data-bs-target", `#${item.id}`);
  tabLink.setAttribute("aria-controls", item.id);
  tabLink.setAttribute("aria-selected", index === 0 ? "true" : "false");
  tabLink.textContent = item.title;

  // Add a data attribute to identify default tool in the UI
  if (item.default) {
    tabLink.setAttribute("data-default", "true");
  }

  // Only the first tab should be active
  if (index !== 0) {
    tabLink.classList.remove("active");
  }

  template.parentNode.appendChild(newTab);
  return newTab;
}

/**
 * Creates a content pane for a tool
 * @param {HTMLElement} template - The content template element
 * @param {Object} item - The tool configuration
 * @param {number} index - The index of the tool
 * @returns {HTMLElement} The created content element
 */
function createToolContent(template, item, index) {
  const newContent = template.cloneNode(true);
  newContent.id = item.id;
  newContent.setAttribute("aria-labelledby", `${item.id}-tab`);

  // Set iframe attributes
  const iframe = newContent.querySelector("iframe");
  if (iframe) {
    iframe.setAttribute("title", item.title);
    // Add additional attributes to help with security and performance
    iframe.setAttribute("loading", "lazy");
    iframe.setAttribute(
      "sandbox",
      "allow-same-origin allow-scripts allow-popups allow-forms allow-modals",
    );
    iframe.setAttribute("data-tool-id", item.id);
  }

  // Only the first content pane should be active
  if (index !== 0) {
    newContent.classList.remove("active", "show");
  }

  template.parentNode.appendChild(newContent);
  return newContent;
}

/**
 * Polls for configuration updates
 */
function startConfigPolling() {
  // Poll every 5 seconds
  setInterval(async () => {
    try {
      const latestConfig = await fetchConfiguration();
      checkForNewTools(latestConfig);
    } catch (error) {
      console.error("Error polling configuration:", error);
    }
  }, 5000);
}

/**
 * Compares current tools with the latest configuration to find new tools
 * @param {Object} latestConfig - The latest configuration from the server
 */
function checkForNewTools(latestConfig) {
  // Get IDs of existing tools
  const existingToolIds = Array.from(
    document.querySelectorAll(".nav-pills .nav-item"),
  ).map((tab) => tab.id.replace("-tab", ""));

  // Find tools that don't already exist in the UI
  const newTools = latestConfig.tools.filter(
    (tool) => !existingToolIds.includes(tool.id),
  );

  if (newTools.length > 0) {
    console.log(`Found ${newTools.length} new tools, adding to UI...`);
    addNewToolsToUI(newTools);
  }
}

/**
 * Adds new tools to the UI dynamically
 * @param {Array} newTools - Array of new tool configurations
 */
function addNewToolsToUI(newTools) {
  const tabContainer = document.querySelector(".nav-pills");
  const contentContainer = document.querySelector(".tab-content");

  newTools.forEach((item) => {
    // Create tab
    const newTab = document.createElement("li");
    newTab.className = "nav-item";
    newTab.id = `${item.id}-tab`;

    const tabLink = document.createElement("a");
    tabLink.className = "nav-link";
    tabLink.href = `#${item.id}`;
    tabLink.setAttribute("data-bs-toggle", "tab");
    tabLink.setAttribute("data-bs-target", `#${item.id}`);
    tabLink.setAttribute("aria-controls", item.id);
    tabLink.setAttribute("aria-selected", "false");
    tabLink.textContent = item.title;

    // Add a data attribute to identify default tool in the UI
    if (item.default) {
      tabLink.setAttribute("data-default", "true");
    }

    newTab.appendChild(tabLink);

    // Insert default tools at the beginning of the list
    if (item.default) {
      tabContainer.insertBefore(newTab, tabContainer.firstChild);
    } else {
      tabContainer.appendChild(newTab);
    }

    // Create content
    const newContent = document.createElement("div");
    newContent.className = "tab-pane h-100 overflow-hidden";
    newContent.id = item.id;
    newContent.setAttribute("aria-labelledby", `${item.id}-tab`);

    const iframe = document.createElement("iframe");
    iframe.className = "w-100 border-0 h-100";
    iframe.title = item.title;

    // Convert relative URLs to absolute
    const url = item.url.startsWith("/")
      ? new URL(item.url.slice(1), window.location.href).href
      : item.url;

    // Set iframe source when tab is shown
    tabLink.addEventListener("shown.bs.tab", () => {
      iframe.src = url;
    });

    newContent.appendChild(iframe);
    contentContainer.appendChild(newContent);

    // If this is a default tool that was just added, activate it
    if (item.default) {
      // Create a Bootstrap tab instance and show this tab
      const tab = new bootstrap.Tab(tabLink);
      tab.show();
      // Load the iframe content immediately
      iframe.src = url;
    }
  });
}

/**
 * Extracts sandbox UUID from the current URL
 * @returns {string|null} The sandbox UUID or null if not found
 */
function extractSandboxUUID() {
  try {
    const hostname = window.location.hostname;
    const uuid = hostname.split(".")[0];
    // Basic validation - UUID should be 36 characters with hyphens
    if (uuid && uuid.length === 36 && uuid.includes("-")) {
      return uuid;
    }
    return null;
  } catch (error) {
    console.error("Error extracting sandbox UUID:", error);
    return null;
  }
}

/**
 * Fetches sandbox expiration data from API with retry logic
 * @param {string} uuid - The sandbox UUID
 * @returns {Promise<Object|null>} The sandbox data or null if failed
 */
async function fetchSandboxData(uuid) {
  const apiUrl = `https://oqfx3p6il2.execute-api.us-west-2.amazonaws.com/staging1/v1/labs/${uuid}`;

  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const response = await fetch(apiUrl);
      if (!response.ok) {
        throw new Error(
          `API request failed: ${response.status} ${response.statusText}`,
        );
      }
      const data = await response.json();
      return data;
    } catch (error) {
      console.error(`Sandbox API attempt ${attempt + 1} failed:`, error);
      if (attempt === 2) {
        // Last attempt failed
        return null;
      }
    }
  }
  return null;
}

/**
 * Starts the countdown timer based on sandbox expiration
 */
async function startCountdownTimer() {
  const timerDisplay = document.getElementById("timer-display");
  if (!timerDisplay) {
    console.error("Timer display element not found");
    return;
  }

  // Show loading state
  timerDisplay.textContent = "--:--";

  // Extract sandbox UUID from URL
  const sandboxUUID = extractSandboxUUID();
  if (!sandboxUUID) {
    console.error("Could not extract sandbox UUID from URL");
    return;
  }

  // Fetch sandbox data
  const sandboxData = await fetchSandboxData(sandboxUUID);
  if (!sandboxData || !sandboxData.expires_at) {
    console.error("Could not fetch sandbox expiration data");
    return;
  }

  // Parse expiration time
  const expirationTime = new Date(sandboxData.expires_at);
  if (isNaN(expirationTime.getTime())) {
    console.error("Invalid expiration time format");
    return;
  }

  // Update timer every second
  const intervalId = setInterval(() => {
    const now = new Date();
    const timeRemaining = Math.max(
      0,
      Math.floor((expirationTime - now) / 1000),
    );

    updateTimerDisplay(timerDisplay, timeRemaining);
    updateTimerStyling(timerDisplay, timeRemaining);

    // Stop timer when expired
    if (timeRemaining <= 0) {
      clearInterval(intervalId);
    }
  }, 1000);

  // Store interval ID for potential cleanup
  window.countdownInterval = intervalId;

  // Update immediately
  const now = new Date();
  const timeRemaining = Math.max(0, Math.floor((expirationTime - now) / 1000));
  updateTimerDisplay(timerDisplay, timeRemaining);
  updateTimerStyling(timerDisplay, timeRemaining);
}

/**
 * Updates the timer display with formatted time
 * @param {HTMLElement} element - The timer display element
 * @param {number} totalSeconds - Total seconds remaining
 */
function updateTimerDisplay(element, totalSeconds) {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  // Format with leading zeros
  const formattedTime = `${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")}`;
  element.textContent = formattedTime;
}

/**
 * Updates timer styling based on remaining time
 * @param {HTMLElement} element - The timer display element
 * @param {number} totalSeconds - Total seconds remaining
 */
function updateTimerStyling(element, totalSeconds) {
  const parentDiv = element.parentElement;

  // Remove all existing color classes
  parentDiv.classList.remove("text-muted", "text-warning", "text-danger");

  // Keep timer grey throughout the entire duration
  parentDiv.classList.add("text-muted");
}

// Initialize the application
initializeLabUI();
