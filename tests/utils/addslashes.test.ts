import { describe, it, expect } from 'vitest';
import { addslashes } from '../../src/utils/addslashes';

describe('addslashes', () => {
    it('escapes quotes, backslashes, backticks and common control chars', () => {
        const input = `line1\nline2\t"double" 'single' \`back\` \\ backslash`;
        const out = addslashes(input);

        expect(out).toContain('\\n');
        expect(out).toContain('\\t');
        expect(out).toContain('\\"');
        expect(out).toContain("\\'");
        expect(out).toContain('\\`');
        expect(out).toContain('\\\\');
    });

    it('stringifies finite numbers', () => {
        expect(addslashes(42)).toBe('42');
        expect(addslashes(-1.5)).toBe('-1.5');
    });

    it('throws on NaN', () => {
        expect(() => addslashes(Number.NaN)).toThrow('NaN cannot be used');
    });
});
