import { describe, it, expect } from 'vitest';
import { escapeRegex, parsePayload } from '../../src/utils/regexp';

describe('escapeRegex', () => {
    it('escapes every regex metacharacter', () => {
        const input = '/\\^$*+?.()|[]{}-';
        const escaped = escapeRegex(input);

        expect(new RegExp(escaped).test(input)).toBe(true);
    });

    it('leaves plain alphanumeric strings untouched', () => {
        expect(escapeRegex('hello123')).toBe('hello123');
    });
});

describe('parsePayload', () => {
    it('returns undefined on empty or falsy input', () => {
        expect(parsePayload(undefined)).toBeUndefined();
        expect(parsePayload('')).toBeUndefined();
    });

    it('extracts action without payload', () => {
        const out = parsePayload('open');
        expect(out?.action).toBe('open');
        expect(out?.data).toBeNull();
    });

    it('extracts action and parses a JSON object payload', () => {
        const out = parsePayload('open{"id":42,"flag":true}');
        expect(out?.action).toBe('open');
        expect(out?.data).toEqual({ id: 42, flag: true });
    });

    it('extracts action and parses a JSON array payload', () => {
        const out = parsePayload('open[1,2,3]');
        expect(out?.action).toBe('open');
        expect(out?.data).toEqual([1, 2, 3]);
    });
});
