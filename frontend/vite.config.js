import { defineConfig } from "vite";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

const __dirname = dirname(fileURLToPath(import.meta.url));

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [wails("./bindings"), tailwindcss()],
  build: {
    rollupOptions: {
      input: {
        // https://vite.dev/guide/build.html#multi-page-app
        index: resolve(__dirname, "index.html"),
        b: resolve(__dirname, "b.html"),
      }
    }
  },
});
