import { defineConfig } from "vite";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  // port
  server: {
    port: parseInt(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [wails("./bindings")],
});
