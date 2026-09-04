module.exports = {
  root: true,
  env: { browser: true, es2021: true, node: true },
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: { jsx: true },
  },
  plugins: ['react-hooks', 'react-refresh'],
  extends: ['eslint:recommended', 'plugin:react-hooks/recommended'],
  ignorePatterns: ['dist', 'node_modules', 'src/.umi*'],
  rules: {
    'no-empty': ['error', { allowEmptyCatch: true }],
    'no-unused-vars': 'off',
    'no-undef': 'off',
    'react-refresh/only-export-components': 'off',
  },
};
