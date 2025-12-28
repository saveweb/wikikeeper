// vite.config.ts
import { sveltekit } from "file:///root/wikitracker/frontend/node_modules/.pnpm/@sveltejs+kit@2.49.2_@sveltejs+vite-plugin-svelte@4.0.4_svelte@5.46.1_vite@5.4.21_sass@_6c93293f041321dab2585722a0fb1a71/node_modules/@sveltejs/kit/src/exports/vite/index.js";
import { defineConfig } from "file:///root/wikitracker/frontend/node_modules/.pnpm/vite@5.4.21_sass@1.97.1/node_modules/vite/dist/node/index.js";
var vite_config_default = defineConfig({
  plugins: [sveltekit()],
  server: {
    port: 5173,
    strictPort: false,
    proxy: {
      "/api": {
        target: "http://localhost:8000",
        changeOrigin: true
      }
    }
  }
});
export {
  vite_config_default as default
};
//# sourceMappingURL=data:application/json;base64,ewogICJ2ZXJzaW9uIjogMywKICAic291cmNlcyI6IFsidml0ZS5jb25maWcudHMiXSwKICAic291cmNlc0NvbnRlbnQiOiBbImNvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9kaXJuYW1lID0gXCIvcm9vdC93aWtpdHJhY2tlci9mcm9udGVuZFwiO2NvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9maWxlbmFtZSA9IFwiL3Jvb3Qvd2lraXRyYWNrZXIvZnJvbnRlbmQvdml0ZS5jb25maWcudHNcIjtjb25zdCBfX3ZpdGVfaW5qZWN0ZWRfb3JpZ2luYWxfaW1wb3J0X21ldGFfdXJsID0gXCJmaWxlOi8vL3Jvb3Qvd2lraXRyYWNrZXIvZnJvbnRlbmQvdml0ZS5jb25maWcudHNcIjtpbXBvcnQgeyBzdmVsdGVraXQgfSBmcm9tICdAc3ZlbHRlanMva2l0L3ZpdGUnO1xuaW1wb3J0IHsgZGVmaW5lQ29uZmlnIH0gZnJvbSAndml0ZSc7XG5cbmV4cG9ydCBkZWZhdWx0IGRlZmluZUNvbmZpZyh7XG5cdHBsdWdpbnM6IFtzdmVsdGVraXQoKV0sXG5cdHNlcnZlcjoge1xuXHRcdHBvcnQ6IDUxNzMsXG5cdFx0c3RyaWN0UG9ydDogZmFsc2UsXG5cdFx0cHJveHk6IHtcblx0XHRcdCcvYXBpJzoge1xuXHRcdFx0XHR0YXJnZXQ6ICdodHRwOi8vbG9jYWxob3N0OjgwMDAnLFxuXHRcdFx0XHRjaGFuZ2VPcmlnaW46IHRydWVcblx0XHRcdH1cblx0XHR9XG5cdH1cbn0pO1xuIl0sCiAgIm1hcHBpbmdzIjogIjtBQUFnUSxTQUFTLGlCQUFpQjtBQUMxUixTQUFTLG9CQUFvQjtBQUU3QixJQUFPLHNCQUFRLGFBQWE7QUFBQSxFQUMzQixTQUFTLENBQUMsVUFBVSxDQUFDO0FBQUEsRUFDckIsUUFBUTtBQUFBLElBQ1AsTUFBTTtBQUFBLElBQ04sWUFBWTtBQUFBLElBQ1osT0FBTztBQUFBLE1BQ04sUUFBUTtBQUFBLFFBQ1AsUUFBUTtBQUFBLFFBQ1IsY0FBYztBQUFBLE1BQ2Y7QUFBQSxJQUNEO0FBQUEsRUFDRDtBQUNELENBQUM7IiwKICAibmFtZXMiOiBbXQp9Cg==
