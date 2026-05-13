const LOVART_URL = "https://www.lovart.ai/";
const COMPLETE_PATH = "/lovart/auth/complete";
let lastHeaders = {};

chrome.webRequest.onBeforeSendHeaders.addListener(
  (details) => {
    const headers = details.requestHeaders || [];
    headers.forEach((header) => {
      const name = String(header.name || "").toLowerCase();
      const value = header.value || "";
      if (!value) {
        return;
      }
      if (name === "token" || name === "authorization" || name === "x-auth-token" || name === "x-access-token") {
        lastHeaders.token = value;
      }
      if (name === "x-csrf-token" || name === "x-xsrf-token" || name === "csrf-token") {
        lastHeaders.csrf = value;
      }
    });
  },
  { urls: ["https://www.lovart.ai/*", "https://*.lovart.ai/*"] },
  ["requestHeaders", "extraHeaders"]
);

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!message || message.type !== "lovart-connect") {
    return false;
  }
  connectToCLI(message)
    .then((result) => sendResponse(result))
    .catch((error) => sendResponse({ ok: false, error: String(error && error.message ? error.message : error) }));
  return true;
});

async function connectToCLI(message) {
  const storage = message.storage || {};
  const cookie = await cookieHeader();
  const cookieHints = parseCookieHints(cookie);
  const payload = {
    state: message.state,
    cookie,
    token: firstNonEmpty(cookieHints.usertoken, storage.token, lastHeaders.token),
    csrf: firstNonEmpty(storage.csrf, lastHeaders.csrf),
    project_id: storage.project_id || "",
    cid: firstNonEmpty(storage.cid, cookieHints.webid),
    source: "browser_extension"
  };
  const response = await fetch("http://127.0.0.1:" + message.port + COMPLETE_PATH, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || "CLI rejected connection");
  }
  return { ok: true };
}

async function cookieHeader() {
  const cookies = await chrome.cookies.getAll({ url: LOVART_URL });
  return cookies
    .map((cookie) => cookie.name + "=" + cookie.value)
    .sort()
    .join("; ");
}

function parseCookieHints(header) {
  const hints = {};
  String(header || "").split(";").forEach((part) => {
    const trimmed = part.trim();
    const equal = trimmed.indexOf("=");
    if (equal <= 0) {
      return;
    }
    const name = trimmed.slice(0, equal);
    const value = trimmed.slice(equal + 1);
    if (name === "usertoken") {
      hints.usertoken = value;
    }
    if (name === "webid") {
      hints.webid = value;
    }
  });
  return hints;
}

function firstNonEmpty() {
  for (let i = 0; i < arguments.length; i += 1) {
    if (arguments[i]) {
      return arguments[i];
    }
  }
  return "";
}
