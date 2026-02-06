import assert from 'assert';
import { getLogContext, runWithLogContext, setLogContext } from '../src/logging/context';

async function testNestedContext() {
    await runWithLogContext({ traceId: 'root', requestId: 'req-1' }, async () => {
        assert.strictEqual(getLogContext().traceId, 'root');

        await runWithLogContext({ updateId: 'u-1' }, async () => {
            const ctx = getLogContext();
            assert.strictEqual(ctx.traceId, 'root');
            assert.strictEqual(ctx.requestId, 'req-1');
            assert.strictEqual(ctx.updateId, 'u-1');

            setLogContext({ commandId: 'cmd-1' });
            assert.strictEqual(getLogContext().commandId, 'cmd-1');
        });

        const after = getLogContext();
        assert.strictEqual(after.updateId, undefined);
        assert.strictEqual(after.commandId, undefined);
        assert.strictEqual(after.traceId, 'root');
    });
}

async function testParallelIsolation() {
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
    assert.deepStrictEqual(values, ['A', 'B']);
}

(async () => {
    await testNestedContext();
    await testParallelIsolation();
    console.log('loggingContextTest: ok');
})();
