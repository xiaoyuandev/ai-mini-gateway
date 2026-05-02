import { defineConfig, loadEnv } from "vite"
import react from "@vitejs/plugin-react"

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "")
  const proxyTarget = env.VITE_DEV_PROXY_TARGET || "http://127.0.0.1:3457"

  return {
    plugins: [react()],
    server: {
      port: 5173,
      proxy: {
        "/health": {
          target: proxyTarget,
          changeOrigin: true
        },
        "/capabilities": {
          target: proxyTarget,
          changeOrigin: true
        },
        "/v1": {
          target: proxyTarget,
          changeOrigin: true
        },
        "/admin": {
          target: proxyTarget,
          changeOrigin: true
        }
      }
    }
  }
})
