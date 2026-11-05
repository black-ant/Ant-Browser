import { lazy } from "react";
import type { ComponentType } from "react";

const CHUNK_RELOAD_COOLDOWN_MS = 10000;
const CHUNK_RELOAD_TS_KEY = "__ant_chunk_reload_ts__";

function isDynamicImportFetchError(error: unknown) {
  const message =
    error instanceof Error ? error.message : String(error ?? "");
  return /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module/i.test(
    message,
  );
}

function reloadForStaleChunkOnce() {
  if (typeof window === "undefined") {
    return false;
  }

  const now = Date.now();
  try {
    const lastAttempt = Number(
      window.sessionStorage.getItem(CHUNK_RELOAD_TS_KEY) || "0",
    );
    if (Number.isFinite(lastAttempt) && now - lastAttempt < CHUNK_RELOAD_COOLDOWN_MS) {
      return false;
    }
    window.sessionStorage.setItem(CHUNK_RELOAD_TS_KEY, String(now));
  } catch {
    // ignore sessionStorage failures and still try a hard reload
  }

  window.location.reload();
  return true;
}

export function lazyNamed<TModule extends Record<string, ComponentType<any>>>(
  loader: () => Promise<TModule>,
  exportName: keyof TModule,
) {
  return lazy(async () => {
    let module: TModule;
    try {
      module = await loader();
    } catch (error) {
      if (isDynamicImportFetchError(error) && reloadForStaleChunkOnce()) {
        return new Promise<never>(() => {});
      }
      throw error;
    }
    return {
      default: module[exportName] as ComponentType<any>,
    };
  });
}
