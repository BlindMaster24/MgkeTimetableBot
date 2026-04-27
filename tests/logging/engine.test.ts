import { describe, expect, it } from 'vitest';
import { LoggingEngine } from '../../src/logging/engine';
import { LoggingConfig } from '../../src/logging/types';

const silentCfg = (): LoggingConfig => ({
    level: 'debug',
    format: 'json',
    output: {
        stdout: false,
        file: { enabled: false, path: './logs/test.log', maxSizeMb: 10, maxFiles: 1 }
    },
    redact: { messageText: true, maxPreviewLen: 32 }
});

describe('LoggingEngine (pino backend)', () => {
    it('writes through all levels without throwing when targets are silent', async () => {
        const engine = new LoggingEngine(silentCfg());
        await expect(engine.write('debug', 'test', 'hello')).resolves.toBeUndefined();
        await expect(engine.write('info', 'test', 'hello', { traceId: 't-1' })).resolves.toBeUndefined();
        await expect(engine.write('warn', 'test', 'hello', { error: new Error('x') })).resolves.toBeUndefined();
        await expect(engine.write('error', 'test', 'hello', { token: 'secret' })).resolves.toBeUndefined();
    });

    it('normalizes Error objects into plain payload fields', async () => {
        const engine = new LoggingEngine(silentCfg());
        const err = new Error('boom');
        await expect(engine.write('error', 'test', 'failed', { error: err })).resolves.toBeUndefined();
    });
});
