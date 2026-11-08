/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#eef8ff',
          100: '#d8efff',
          200: '#b9e3ff',
          300: '#89d2fe',
          400: '#52b8fc',
          500: '#2a9af6',
          600: '#147ceb',
          700: '#0f63d0',
          800: '#1350a8',
          900: '#154485',
        },
      },
    },
  },
  plugins: [],
}
