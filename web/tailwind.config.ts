import type { Config } from "tailwindcss"

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: [
          "Avenir Next",
          "Segoe UI",
          "PingFang SC",
          "Hiragino Sans GB",
          "ui-sans-serif",
          "system-ui",
          "sans-serif"
        ],
        display: [
          "Iowan Old Style",
          "Palatino Linotype",
          "Noto Serif SC",
          "serif"
        ],
        mono: [
          "SFMono-Regular",
          "Menlo",
          "Monaco",
          "Cascadia Code",
          "ui-monospace",
          "monospace"
        ]
      },
      boxShadow: {
        paper: "0 22px 80px rgba(74, 44, 14, 0.14)"
      },
      colors: {
        lagoon: {
          50: "#eff9f9",
          500: "#1f6f78",
          700: "#164d54"
        },
        rust: {
          500: "#b85c38",
          700: "#8a4125"
        }
      }
    }
  },
  plugins: []
} satisfies Config
