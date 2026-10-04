import type { Config } from "@react-router/dev/config";

export default {
  // Config options...
  // Static export: build generates build/client/index.html for the SPA
  ssr: false,
  // Scan every route before serving so lazy navigation cannot introduce a new
  // dependency bundle alongside an already-loaded copy of React.
  future: {
    unstable_optimizeDeps: true,
  },
} satisfies Config;
