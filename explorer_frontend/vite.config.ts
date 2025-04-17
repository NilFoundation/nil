import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import vitePluginString from "vite-plugin-string";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    port: Number(process.env.PORT) || 5173,
  },
  plugins: [
    react(),
    vitePluginString({
      include: ["**/*.sol", "**/*.md"],
      compress: false,
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
            `${scriptTag[0].replace("<script", "<script defer")}</body>`
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
