(function () {
  const PORT_START = 47821;
  const PORT_END = 47830;
  const SESSION_PATH = "/lovart/auth/session";
  const CANCEL_PATH = "/lovart/auth/cancel";
  const POLL_MS = 1500;
  const FETCH_TIMEOUT_MS = 350;

  let activeSession = null;
  let dismissedState = "";
  let connecting = false;
  let scanning = false;

  function timeoutSignal(ms) {
    const controller = new AbortController();
    setTimeout(() => controller.abort(), ms);
    return controller.signal;
  }

  async function fetchJSON(url, options) {
    const response = await fetch(url, {
      cache: "no-store",
      signal: timeoutSignal(FETCH_TIMEOUT_MS),
      ...options
    });
    if (!response.ok) {
      throw new Error("HTTP " + response.status);
    }
    return response.json();
  }

  async function findSession() {
    for (let port = PORT_START; port <= PORT_END; port += 1) {
      try {
        const session = await fetchJSON("http://127.0.0.1:" + port + SESSION_PATH);
        if (session && session.state && session.state !== dismissedState) {
          return { port, state: session.state, expires_at: session.expires_at };
        }
      } catch (_) {
        // Keep scanning other ports.
      }
    }
    return null;
  }

  function collectStorageHints() {
    const hints = {};
    scanStorage(window.localStorage, hints);
    scanStorage(window.sessionStorage, hints);
    return hints;
  }

  function scanStorage(storage, hints) {
    if (!storage) {
      return;
    }
    for (let i = 0; i < storage.length; i += 1) {
      const key = storage.key(i) || "";
      const value = storage.getItem(key) || "";
      inspectPair(key, value, hints);
      inspectJSON(value, hints);
    }
  }

  function inspectJSON(value, hints) {
    if (!value || value.length > 20000) {
      return;
    }
    try {
      walk(JSON.parse(value), hints);
    } catch (_) {
      // Non-JSON values are common in browser storage.
    }
  }

  function walk(value, hints) {
    if (!value || typeof value !== "object") {
      return;
    }
    if (Array.isArray(value)) {
      value.forEach((item) => walk(item, hints));
      return;
    }
    Object.keys(value).forEach((key) => {
      const item = value[key];
      if (typeof item === "string") {
        inspectPair(key, item, hints);
      } else {
        walk(item, hints);
      }
    });
  }

  function inspectPair(key, value, hints) {
    const lower = String(key).toLowerCase();
    if (!hints.token && (lower === "token" || lower.includes("auth_token") || lower.includes("access_token"))) {
      hints.token = value;
    }
    if (!hints.csrf && (lower.includes("csrf") || lower.includes("xsrf"))) {
      hints.csrf = value;
    }
    if (!hints.project_id && (lower === "project_id" || lower === "projectid" || lower.includes("project_id"))) {
      hints.project_id = value;
    }
    if (!hints.cid && (lower === "cid" || lower === "webid" || lower === "web_id")) {
      hints.cid = value;
    }
  }

  function showPrompt(session) {
    activeSession = session;
    let root = document.getElementById("lovart-cli-connect");
    if (!root) {
      root = document.createElement("div");
      root.id = "lovart-cli-connect";
      root.innerHTML = [
        '<p class="lovart-cli-title">Lovart CLI wants to connect</p>',
        '<p class="lovart-cli-body">Share your signed-in Lovart browser session with the local CLI on this machine.</p>',
        '<div class="lovart-cli-actions">',
        '<button type="button" data-action="dismiss">Not now</button>',
        '<button type="button" data-primary="true" data-action="connect">Connect</button>',
        "</div>",
        '<div class="lovart-cli-status" aria-live="polite"></div>'
      ].join("");
      document.documentElement.appendChild(root);
      root.addEventListener("click", handlePromptClick);
    }
    setStatus("");
  }

  function hidePrompt() {
    const root = document.getElementById("lovart-cli-connect");
    if (root) {
      root.remove();
    }
    activeSession = null;
  }

  function setStatus(text) {
    const status = document.querySelector("#lovart-cli-connect .lovart-cli-status");
    if (status) {
      status.textContent = text || "";
    }
  }

  async function handlePromptClick(event) {
    const action = event.target && event.target.getAttribute("data-action");
    if (!action || !activeSession) {
      return;
    }
    if (action === "dismiss") {
      dismissedState = activeSession.state;
      await cancelSession(activeSession);
      hidePrompt();
      return;
    }
    if (action === "connect") {
      await connectSession(activeSession);
    }
  }

  async function cancelSession(session) {
    try {
      await fetch("http://127.0.0.1:" + session.port + CANCEL_PATH, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ state: session.state })
      });
    } catch (_) {
      // CLI may already be gone.
    }
  }

  async function connectSession(session) {
    if (connecting) {
      return;
    }
    connecting = true;
    setStatus("Connecting...");
    chrome.runtime.sendMessage(
      {
        type: "lovart-connect",
        port: session.port,
        state: session.state,
        storage: collectStorageHints()
      },
      (response) => {
        connecting = false;
        if (chrome.runtime.lastError) {
          setStatus(chrome.runtime.lastError.message);
          return;
        }
        if (response && response.ok) {
          setStatus("Connected. Return to your terminal.");
          setTimeout(hidePrompt, 1200);
          return;
        }
        setStatus((response && response.error) || "Connection failed.");
      }
    );
  }

  async function scanAndPrompt() {
    if (activeSession || connecting || scanning) {
      return;
    }
    scanning = true;
    try {
      const session = await findSession();
      if (session) {
        showPrompt(session);
      }
    } finally {
      scanning = false;
    }
  }

  chrome.runtime.onMessage.addListener((message) => {
    if (message && message.type === "lovart-manual-scan") {
      dismissedState = "";
      scanAndPrompt();
    }
  });

  scanAndPrompt();
  setInterval(scanAndPrompt, POLL_MS);
})();
