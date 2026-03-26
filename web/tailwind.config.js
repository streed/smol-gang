/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Cyberpunk color palette
        cyber: {
          bg: '#0a0a0f',
          surface: '#12121a',
          card: '#1a1a2e',
          border: '#2a2a3e',
          hover: '#22223a',
        },
        neon: {
          cyan: '#00f0ff',
          green: '#39ff14',
          magenta: '#ff00ff',
          pink: '#ff2d7b',
          yellow: '#ffd700',
          orange: '#ff6a00',
          purple: '#b44dff',
          red: '#ff0040',
        },
      },
      fontFamily: {
        mono: ['"JetBrains Mono"', '"Fira Code"', 'monospace'],
        display: ['"Orbitron"', '"Rajdhani"', 'sans-serif'],
        sans: ['"Inter"', '"Segoe UI"', 'sans-serif'],
      },
      boxShadow: {
        'neon-cyan': '0 0 5px #00f0ff, 0 0 20px rgba(0, 240, 255, 0.15)',
        'neon-green': '0 0 5px #39ff14, 0 0 20px rgba(57, 255, 20, 0.15)',
        'neon-magenta': '0 0 5px #ff00ff, 0 0 20px rgba(255, 0, 255, 0.15)',
        'neon-pink': '0 0 5px #ff2d7b, 0 0 20px rgba(255, 45, 123, 0.15)',
        'neon-red': '0 0 5px #ff0040, 0 0 20px rgba(255, 0, 64, 0.15)',
        'glow': '0 0 15px rgba(0, 240, 255, 0.1)',
      },
      backgroundImage: {
        'grid-pattern': 'linear-gradient(rgba(0, 240, 255, 0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(0, 240, 255, 0.03) 1px, transparent 1px)',
        'gradient-radial': 'radial-gradient(ellipse at center, var(--tw-gradient-stops))',
      },
      backgroundSize: {
        'grid': '24px 24px',
      },
      animation: {
        'pulse-slow': 'pulse 3s ease-in-out infinite',
        'glow': 'glow 2s ease-in-out infinite alternate',
        'scan': 'scan 4s linear infinite',
      },
      keyframes: {
        glow: {
          '0%': { boxShadow: '0 0 5px rgba(0, 240, 255, 0.2)' },
          '100%': { boxShadow: '0 0 20px rgba(0, 240, 255, 0.4)' },
        },
        scan: {
          '0%': { backgroundPosition: '0% 0%' },
          '100%': { backgroundPosition: '0% 100%' },
        },
      },
    },
  },
  plugins: [],
};
