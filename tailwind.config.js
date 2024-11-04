export default {
  content: ["./web/public/**/*.html", "./web/views/**/*.templ"],
  theme: {
    extend: {},
  },
  plugins: [require("@tailwindcss/typography")],
};
