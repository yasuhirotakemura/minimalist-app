import js from '@eslint/js'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.ts', '**/*.vue'],
  },
  {
    name: 'app/files-to-ignore',
    ignores: ['dist/**', 'coverage/**', 'src/api/generated/**', 'node_modules/**'],
  },
  js.configs.recommended,
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,
  skipFormatting,
  {
    name: 'app/rules',
    rules: {
      // 設計書 10.4: API responseをPiniaへ複製しない、tokenをlocalStorageへ保存しない。
      // 静的解析で機械的に検出できる範囲を禁止する。
      'no-restricted-globals': [
        'error',
        {
          name: 'localStorage',
          message:
            '認証tokenをlocalStorageへ保存しない (設計書 10.4)。sessionはhttpOnly Cookieで管理する。',
        },
        {
          name: 'sessionStorage',
          message: '認証情報をsessionStorageへ保存しない (設計書 10.4)。',
        },
      ],
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      'vue/multi-word-component-names': 'off',
      'vue/component-api-style': ['error', ['script-setup']],
      'vue/define-macros-order': ['error', { order: ['defineProps', 'defineEmits'] }],
      'vue/no-mutating-props': 'error',
    },
  },
  {
    name: 'app/test-overrides',
    files: ['tests/**/*.ts'],
    rules: {
      'no-restricted-globals': 'off',
    },
  },
)
