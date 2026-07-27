/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        cyber: {
          black: '#0a0a0f',
          dark: '#12101a',
          deeper: '#08060e',
          card: '#1a1625',
          border: '#2a2438',
          accent: '#6c3bff',
          cyan: '#00f0ff',
          magenta: '#ff2d95',
          green: '#00ff88',
          red: '#ff0040',
          amber: '#ffaa00',
          text: '#c8c0d8',
          dim: '#7a7290',
        },
      },
      fontFamily: {
        mono: ['"JetBrains Mono"', 'Fira Code', 'monospace'],
        display: ['"RulesCompressed"', 'sans-serif'],
      },
      animation: {
        glitch: 'glitch 2s infinite',
        pulse_glow: 'pulseGlow 2s ease-in-out infinite',
        scan: 'scan 3s linear infinite',
      },
      keyframes: {
        glitch: {
          '0%, 100%': { transform: 'translate(0)' },
          '20%': { transform: 'translate(-2px, 1px)' },
          '40%': { transform: 'translate(2px, -1px)' },
          '60%': { transform: 'translate(-1px, 2px)' },
          '80%': { transform: 'translate(1px, -1px)' },
        },
        pulseGlow: {
          '0%, 100%': { opacity: '0.6' },
          '50%': { opacity: '1' },
        },
        scan: {
          '0%': { transform: 'translateY(-100%)' },
          '100%': { transform: 'translateY(100%)' },
        },
      },
      boxShadow: {
        neon: '0 0 10px rgba(108, 59, 255, 0.5), 0 0 40px rgba(108, 59, 255, 0.2)',
        cyan: '0 0 10px rgba(0, 240, 255, 0.4), 0 0 30px rgba(0, 240, 255, 0.1)',
        magenta: '0 0 10px rgba(255, 45, 149, 0.4), 0 0 30px rgba(255, 45, 149, 0.1)',
      },
    },
  },
  plugins: [],
}
