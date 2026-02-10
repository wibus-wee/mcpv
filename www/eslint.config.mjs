// @ts-check
import { defineConfig } from 'eslint-config-hyoban'

export default defineConfig(
  {
    lessOpinionated: true,
    preferESM: false,
    react: true,
    tailwindCSS: false,
    ignores: ['**/api-gen/**', '**/components/ui/**', '**/bindings/**', '**/routeTree.gen.ts'],
  },
  {
    settings: {
      tailwindcss: {
        whitelist: ['center'],
      },
    },
    rules: {
      'unicorn/prefer-math-trunc': 'off',
      '@eslint-react/no-clone-element': 0,
      '@eslint-react/hooks-extra/no-direct-set-state-in-use-effect': 0,
      'no-restricted-syntax': 0,
      'react-google-translate/no-conditional-text-nodes-with-siblings': 0,
    },
  },
  {
    files: ['**/*.tsx'],
    rules: {
      '@stylistic/jsx-self-closing-comp': 'error',
    },
  },
)
