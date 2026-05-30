import js from '@eslint/js';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import reactHooks from 'eslint-plugin-react-hooks';
import security from 'eslint-plugin-security';
import tseslint from 'typescript-eslint';

const browserGlobals = {
  Blob: 'readonly',
  URL: 'readonly',
  console: 'readonly',
  document: 'readonly',
  localStorage: 'readonly',
  navigator: 'readonly',
  window: 'readonly',
};

const nodeGlobals = {
  process: 'readonly',
};

export default tseslint.config(
  {
    ignores: [
      '**/node_modules/**',
      '**/dist/**',
      '**/.next/**',
      '**/.vite/**',
      '**/coverage/**',
      '**/tmp/**',
      'core/**',
      'infra/**',
      'templates/**',
      'clients/web/**',
      'clients/vscode-extension/**',
    ],
    linterOptions: {
      reportUnusedDisableDirectives: false,
    },
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['clients/studio/**/*.{ts,tsx}', 'packages/**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: { ...browserGlobals, ...nodeGlobals },
    },
    plugins: {
      'jsx-a11y': jsxA11y,
      'react-hooks': reactHooks,
      security,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      ...jsxA11y.flatConfigs.recommended.rules,
      'jsx-a11y/no-autofocus': 'off',
      'react-hooks/exhaustive-deps': 'off',
      'react-hooks/purity': 'off',
      'react-hooks/set-state-in-effect': 'off',
      'security/detect-object-injection': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      'no-empty': ['error', { allowEmptyCatch: true }],
      'no-restricted-syntax': [
        'error',
        {
          selector: 'CallExpression[callee.object.name="localStorage"][callee.property.name=/^(setItem|getItem)$/]',
          message: 'Use the shared auth/runtime storage adapter so token handling stays auditable.',
        },
      ],
    },
  },
  {
    files: ['**/*.test.{ts,tsx}', '**/test-setup.ts', 'clients/studio/src/lib/authToken.ts'],
    rules: {
      'no-restricted-syntax': 'off',
    },
  },
);
