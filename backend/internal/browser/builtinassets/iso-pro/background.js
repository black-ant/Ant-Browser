async function refreshBadge(launches) {
  await chrome.action.setBadgeText({ text: String(launches || 0) });
  await chrome.action.setBadgeBackgroundColor({ color: "#2563eb" });
}

async function bumpLaunches() {
  const state = await chrome.storage.local.get({
    launches: 0,
    fingerprint: "",
    firstSeen: "",
  });
  const next = {
    launches: Number(state.launches || 0) + 1,
    fingerprint: state.fingerprint || crypto.randomUUID(),
    firstSeen: state.firstSeen || new Date().toISOString(),
  };
  await chrome.storage.local.set(next);
  await refreshBadge(next.launches);
}

chrome.runtime.onInstalled.addListener(() => {
  void bumpLaunches();
});

chrome.runtime.onStartup.addListener(() => {
  void bumpLaunches();
});

void chrome.storage.local.get({ launches: 0 }).then((state) => refreshBadge(state.launches));
