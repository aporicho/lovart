document.getElementById("scan").addEventListener("click", async () => {
  const status = document.getElementById("status");
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  const tab = tabs[0];
  if (!tab || !tab.id || !/^https:\/\/([^/]+\.)?lovart\.ai\//.test(tab.url || "")) {
    status.textContent = "Open a Lovart page first.";
    return;
  }
  chrome.tabs.sendMessage(tab.id, { type: "lovart-manual-scan" }, () => {
    if (chrome.runtime.lastError) {
      status.textContent = "Refresh the Lovart page and try again.";
      return;
    }
    status.textContent = "Check the Lovart page for the Connect prompt.";
  });
});
