import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import unusedImports from 'eslint-plugin-unused-imports';
import prettierConfig from 'eslint-config-prettier';
import globals from 'globals';

export default tseslint.config(
    {
        ignores: [
            'node_modules/**',
            'coverage/**',
            'cache/**',
            'dist/**',
            'build/**',
            'public/**',
            '*.js',
            '*.mjs',
            '*.cjs',
            '!eslint.config.mjs'
        ]
    },
    js.configs.recommended,
    ...tseslint.configs.recommended,
    {
        languageOptions: {
            ecmaVersion: 2024,
            sourceType: 'module',
            globals: {
                ...globals.node
            }
        },
        plugins: {
            'unused-imports': unusedImports
        },
        rules: {
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/no-unused-vars': 'off',
            '@typescript-eslint/no-empty-object-type': 'off',
            '@typescript-eslint/no-require-imports': 'off',
            '@typescript-eslint/no-unused-expressions': 'off',
            '@typescript-eslint/no-this-alias': 'off',
            '@typescript-eslint/ban-ts-comment': 'off',
            '@typescript-eslint/no-namespace': 'off',
            '@typescript-eslint/no-wrapper-object-types': 'off',
            '@typescript-eslint/no-unsafe-declaration-merging': 'off',
            '@typescript-eslint/no-empty-function': 'off',
            'no-empty': ['warn', { allowEmptyCatch: true }],
            'no-useless-escape': 'off',
            'no-case-declarations': 'off',
            'no-async-promise-executor': 'off',
            'no-prototype-builtins': 'off',
            'no-constant-binary-expression': 'off',
            'no-constant-condition': ['error', { checkLoops: false }],
            'no-control-regex': 'off',
            'no-misleading-character-class': 'off',
            'prefer-const': ['warn', { destructuring: 'all' }],
            'unused-imports/no-unused-imports': 'warn',
            '@typescript-eslint/no-misused-new': 'off',
            'no-useless-assignment': 'off',
            'no-unsafe-optional-chaining': 'warn',
            'no-cond-assign': ['error', 'except-parens']
        }
    },
    {
        files: ['tests/**/*.ts'],
        rules: {
            '@typescript-eslint/no-explicit-any': 'off',
            'no-empty-pattern': 'off'
        }
    },
    prettierConfig
);
