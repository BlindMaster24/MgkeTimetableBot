import { describe, it, expect } from 'vitest';
import type { Request } from 'express';
import { getIp, getParams, replaceWithValueLength } from '../../src/utils/http';

const makeRequest = (
    query: any = {},
    body: any = {},
    headers: Record<string, string> = {},
    ip = '127.0.0.1'
): Request =>
    ({
        query,
        body,
        ip,
        header(name: string) {
            return headers[name];
        }
    }) as unknown as Request;

describe('getParams', () => {
    it('merges query and body with body overriding query on key conflict', () => {
        const req = makeRequest({ a: 1, shared: 'q' }, { b: 2, shared: 'b' });
        expect(getParams(req)).toEqual({ a: 1, b: 2, shared: 'b' });
    });

    it('returns empty object for empty query and body', () => {
        const req = makeRequest();
        expect(getParams(req)).toEqual({});
    });
});

describe('getIp', () => {
    it('returns the first entry from X-Forwarded-For when present', () => {
        const req = makeRequest({}, {}, { 'X-Forwarded-For': '10.0.0.1, 10.0.0.2' });
        expect(getIp(req)).toBe('10.0.0.1');
    });

    it('falls back to req.ip when no X-Forwarded-For header is set', () => {
        const req = makeRequest({}, {}, {}, '192.168.1.1');
        expect(getIp(req)).toBe('192.168.1.1');
    });
});

describe('replaceWithValueLength', () => {
    it('replaces long string and array values with their length', () => {
        const big = 'x'.repeat(40);
        const out = replaceWithValueLength({ short: 'hi', long: big, arr: new Array(50).fill(0) });

        expect(out.short).toBe('hi');
        expect(out.long).toBe(big.length);
        expect(out.arr).toBe(50);
    });

    it('keeps other value types unchanged', () => {
        const out = replaceWithValueLength({ n: 5, flag: true, nested: { a: 1 } });
        expect(out).toEqual({ n: 5, flag: true, nested: { a: 1 } });
    });
});
