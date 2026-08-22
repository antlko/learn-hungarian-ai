import { globalIgnores } from 'eslint/config'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'
import oxlint from 'eslint-plugin-oxlint'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'

export default defineConfigWithVueTs(
  { name: 'app/files-to-lint', files: ['**/*.{ts,mts,tsx,vue}'] },
  globalIgnores(['**/dist/**', '**/dist-ssr/**', '**/coverage/**']),

  pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,

  {
    // shadcn-vue primitives are intentionally single-word (Button, Card, Input).
    // The convention comes from upstream; renaming them would break every import.
    name: 'app/ui-primitives',
    files: ['src/components/ui/**/*.vue'],
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },
  {
    name: 'app/conventions',
    rules: {
      // `const { class: _, ...rest }` is the standard shadcn-vue omit pattern.
      '@typescript-eslint/no-unused-vars': ['error', { varsIgnorePattern: '^_' }],
    },
  },

  oxlint.configs['flat/recommended'],
  skipFormatting,
)
