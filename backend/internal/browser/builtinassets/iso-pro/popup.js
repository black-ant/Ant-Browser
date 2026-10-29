const $ = (id) => document.getElementById(id);

function setStatus(text, ok) {
  const el = $("status");
  el.textContent = text;
  el.className = ok ? "ok" : "bad";
}

function openDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("iso-probe", 1);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains("kv")) db.createObjectStore("kv");
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

async function readIdbCounter() {
  const db = await openDB();
  return new Promise((resolve) => {
    const tx = db.transaction("kv", "readonly");
    const get = tx.objectStore("kv").get("counter");
    get.onsuccess = () => resolve(Number(get.result || 0));
    get.onerror = () => resolve(-1);
  });
}

async function writeIdbCounter(value) {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction("kv", "readwrite");
    tx.objectStore("kv").put(value, "counter");
    tx.oncomplete = () => resolve(value);
    tx.onerror = () => reject(tx.error);
  });
}

function readLocalStorageCounter() {
  return Number(localStorage.getItem("isoCounter") || 0);
}

async function render() {
  const manifest = chrome.runtime.getManifest();
  $("extId").textContent = chrome.runtime.id;
  $("version").textContent = manifest.version;

  const local = await chrome.storage.local.get({
    launches: 0,
    fingerprint: "",
    firstSeen: "",
    mark: "",
    storageLocalCounter: 0,
  });
  $("fingerprint").textContent = local.fingerprint || "(未生成)";
  $("launches").textContent = String(local.launches || 0);
  $("firstSeen").textContent = local.firstSeen || "-";
  $("markValue").textContent = local.mark || "(空)";
  $("cLocal").textContent = String(local.storageLocalCounter || 0);

  const sync = await chrome.storage.sync.get({ storageSyncCounter: 0 });
  $("cSync").textContent = String(sync.storageSyncCounter || 0);

  try {
    $("cIdb").textContent = String(await readIdbCounter());
  } catch (error) {
    $("cIdb").textContent = "读取失败";
  }
  $("cLs").textContent = String(readLocalStorageCounter());
}

async function bumpAll() {
  const local = await chrome.storage.local.get({ storageLocalCounter: 0 });
  await chrome.storage.local.set({ storageLocalCounter: Number(local.storageLocalCounter || 0) + 1 });

  const sync = await chrome.storage.sync.get({ storageSyncCounter: 0 });
  await chrome.storage.sync.set({ storageSyncCounter: Number(sync.storageSyncCounter || 0) + 1 });

  try {
    await writeIdbCounter((await readIdbCounter()) + 1);
  } catch (error) {
    setStatus("IndexedDB 写入失败：" + error.message, false);
  }

  localStorage.setItem("isoCounter", String(readLocalStorageCounter() + 1));
  await render();
}

async function exportSnapshot() {
  const local = await chrome.storage.local.get(null);
  let idb = -1;
  try {
    idb = await readIdbCounter();
  } catch (error) {
    idb = -1;
  }
  const snapshot = {
    extId: chrome.runtime.id,
    version: chrome.runtime.getManifest().version,
    storageLocal: {
      launches: local.launches || 0,
      fingerprint: local.fingerprint || "",
      firstSeen: local.firstSeen || "",
      mark: local.mark || "",
      counter: local.storageLocalCounter || 0,
    },
    storageSyncCounter: (await chrome.storage.sync.get({ storageSyncCounter: 0 })).storageSyncCounter,
    indexedDbCounter: idb,
    localStorageCounter: readLocalStorageCounter(),
    exportedAt: new Date().toISOString(),
  };
  $("snapshot").value = JSON.stringify(snapshot, null, 2);
  setStatus("已生成快照，可直接复制对比", true);
}

async function resetThisInstance() {
  await chrome.storage.local.clear();
  await chrome.storage.sync.clear();
  localStorage.clear();
  try {
    const db = await openDB();
    await new Promise((resolve) => {
      const tx = db.transaction("kv", "readwrite");
      tx.objectStore("kv").clear();
      tx.oncomplete = () => resolve();
      tx.onerror = () => resolve();
    });
  } catch (error) {
    // 忽略
  }
  await render();
  setStatus("本实例数据已清空，其他实例不受影响", true);
}

async function saveMark() {
  const mark = $("mark").value.trim();
  await chrome.storage.local.set({ mark });
  await render();
  setStatus(mark ? `已保存标记：${mark}` : "标记已清空", true);
}

$("bumpAll").addEventListener("click", () => void bumpAll());
$("reset").addEventListener("click", () => void resetThisInstance());
$("export").addEventListener("click", () => void exportSnapshot());
$("saveMark").addEventListener("click", () => void saveMark());

void render();
