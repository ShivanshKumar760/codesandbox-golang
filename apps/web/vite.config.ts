import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In local dev, "/api/..." is forwarded to the real API, so the browser
// never makes a cross-origin call (no CORS needed). On Vercel, vercel.json
// does the same job.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "https://codeapi-101353847207.asia-south1.run.app",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});