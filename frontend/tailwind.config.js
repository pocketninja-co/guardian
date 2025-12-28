/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // SaaS Light Theme Palette
        surface: '#ffffff',
        muted: '#f8fafc', // Slate-50
        border: '#e2e8f0', // Slate-200
        
        primary: {
          DEFAULT: '#0f172a', // Slate-900 (Text)
          light: '#475569',   // Slate-600 (Secondary Text)
        },
        
        accent: {
          blue: '#3b82f6', // Bright Blue
          red: '#ef4444',  // Red for risks
          green: '#10b981', // Emerald for success
          amber: '#f59e0b', // Amber for warnings
        },
        
        // Legacy support mapping
        'medical-blue': '#3b82f6',
        'medical-red': '#ef4444',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
