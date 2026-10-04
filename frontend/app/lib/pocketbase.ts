import PocketBase from "pocketbase";

// PocketBase serves the SPA, so the API is same-origin in production.
// `window` is absent during the build-time prerender, so guard it.
const url =
  import.meta.env?.VITE_PB_URL ||
  (import.meta.env?.DEV ? "http://127.0.0.1:8090" : globalThis.window?.location.origin);

export const pb = new PocketBase(url);

// Each component owns cancellation through its AbortSignal.
pb.autoCancellation(false);
