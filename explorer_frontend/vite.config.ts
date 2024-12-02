import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { nodePolyfills } from "vite-plugin-node-polyfills";
import vitePluginString from "vite-plugin-string";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8529",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
  plugins: [
    react(),
    nodePolyfills({
      include: ["buffer"],
      globals: {
        Buffer: true,
      },
    }),
    vitePluginString({
      include: ["**/*.sol"],
    }),
    {
      name: "custom-script-tag",
      enforce: "post",
      transformIndexHtml(html) {
        let newHtml = html;
        const scriptTagRegex = /<script type="module".*<\/script>/;
        const scriptTag = html.match(scriptTagRegex);

        if (scriptTag) {
          newHtml = newHtml.replace(scriptTagRegex, "");

          // Add defer attribute to the script tag and append it to the end of the body
          newHtml = newHtml.replace(
            "</body>",
            `${scriptTag[0].replace("<script", "<script defer")}</body>`,
          );
        }

        return newHtml;
      },
    },
  ],
  build: {
    sourcemap: true,
    assetsInlineLimit: 14000, // less than 14 KiB
  },
});
