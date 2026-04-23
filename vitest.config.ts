import { defineConfig } from 'vitest/config';

export default defineConfig({
    test: {
        include: ['tests/**/*.test.ts', 'src/**/*.test.ts'],
        environment: 'node',
        globals: false,
        testTimeout: 10000,
        reporters: ['default'],
        coverage: {
            provider: 'v8',
            reporter: ['text', 'html', 'lcov', 'json-summary'],
            reportsDirectory: 'coverage',
            include: ['src/**/*.ts'],
            exclude: ['src/**/*.d.ts', 'src/@types/**', 'src/bootstrap.ts', 'src/index.ts', 'src/app.ts']
        }
    }
});
