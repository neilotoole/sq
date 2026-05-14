import js from "@eslint/js";
import globals from "globals";

export default [
  js.configs.recommended,
  {
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      "no-unused-vars": ["error", { argsIgnorePattern: "^_", caughtErrorsIgnorePattern: "^_" }],
    },
  },
  {
    ignores: [
      "assets/js/index.js",
      "assets/js/katex.js",
      "assets/js/vendor/**",
      "node_modules/**",
      "config/postcss.config.js",
    ],
  },
];
