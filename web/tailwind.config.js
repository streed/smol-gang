/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Dark theme palette — softer than full neon
        cyber: {
          bg: '#0c0c14',
          surface: '#13131e',
          card: '#1a1a2a',
          border: '#2a2a3e',
          hover: '#222236',
        },
        neon: {
          cyan: '#5cc8d4',
          green: '#4ade80',
          magenta: '#c084fc',
          pink: '#f472b6',
          yellow: '#fbbf24',
          orange: '#fb923c',
          purple: '#a78bfa',
          red: '#f87171',
        },
      },
      fontFamily: {
        mono: ['"JetBrains Mono"', '"Fira Code"', 'monospace'],
        display: ['"Inter"', '"Segoe UI"', 'sans-serif'],
        sans: ['"Inter"', '"Segoe UI"', 'sans-serif'],
      },
      boxShadow: {
        'neon-cyan': '0 0 8px rgba(92, 200, 212, 0.12)',
        'neon-green': '0 0 8px rgba(74, 222, 128, 0.12)',
        'neon-magenta': '0 0 8px rgba(192, 132, 252, 0.12)',
        'neon-pink': '0 0 8px rgba(244, 114, 182, 0.12)',
        'neon-red': '0 0 8px rgba(248, 113, 113, 0.12)',
        'glow': '0 0 10px rgba(92, 200, 212, 0.06)',
      },
      backgroundImage: {
        'grid-pattern': 'linear-gradient(rgba(92, 200, 212, 0.02) 1px, transparent 1px), linear-gradient(90deg, rgba(92, 200, 212, 0.02) 1px, transparent 1px)',
        'gradient-radial': 'radial-gradient(ellipse at center, var(--tw-gradient-stops))',
      },
      backgroundSize: {
        'grid': '24px 24px',
      },
      animation: {
        'pulse-slow': 'pulse 3s ease-in-out infinite',
      },
    },
  },
  plugins: [require('@tailwindcss/typography')],
};
