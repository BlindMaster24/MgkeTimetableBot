import { describe, it, expect } from 'vitest';
import { getLogContext, runWithLogContext, setLogContext } from '../../src/logging/context';

describe('logging context (ALS)', () => {
    it('propagates and overrides context through nested async calls', async () => {
        await runWithLogContext({ traceId: 'root', requestId: 'req-1' }, async () => {
            expect(getLogContext().traceId).toBe('root');

            await runWithLogContext({ updateId: 'u-1' }, async () => {
                const ctx = getLogContext();
                expect(ctx.traceId).toBe('root');
                expect(ctx.requestId).toBe('req-1');
                expect(ctx.updateId).toBe('u-1');

                setLogContext({ commandId: 'cmd-1' });
                expect(getLogContext().commandId).toBe('cmd-1');
            });

            const after = getLogContext();
            expect(after.updateId).toBeUndefined();
            expect(after.commandId).toBeUndefined();
            expect(after.traceId).toBe('root');
        });
    });

    it('isolates context between parallel async chains', async () => {
        const values: Array<string | undefined> = [];

        await Promise.all([
            runWithLogContext({ traceId: 'A' }, async () => {
                await new Promise((resolve) => setTimeout(resolve, 10));
                values.push(String(getLogContext().traceId));
            }),
            runWithLogContext({ traceId: 'B' }, async () => {
                await new Promise((resolve) => setTimeout(resolve, 5));
                values.push(String(getLogContext().traceId));
            })
        ]);

        values.sort();
        expect(values).toEqual(['A', 'B']);
    });
});
