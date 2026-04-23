import { describe, it, expect } from 'vitest';
import { prepareError } from '../../src/utils/error';

describe('prepareError', () => {
    it('returns the stack trace with cwd stripped when no parser context is attached', () => {
        const err = new Error('boom');
        const result = prepareError(err);

        expect(typeof result).toBe('string');
        expect(result).toContain('Error: boom');
        expect(result).not.toContain(process.cwd());
    });

    it('appends parserContext JSON when present and non-empty', () => {
        const err = new Error('bad') as any;
        err.parserContext = { groupId: 167, week: 34 };

        const result = prepareError(err)!;

        expect(result).toContain('context:');
        expect(result).toContain('"groupId": 167');
        expect(result).toContain('"week": 34');
    });

    it('omits context section when parserContext is empty object', () => {
        const err = new Error('bad') as any;
        err.parserContext = {};

        const result = prepareError(err)!;

        expect(result).not.toContain('context:');
    });

    it('ignores non-object parserContext values', () => {
        const err = new Error('bad') as any;
        err.parserContext = 'unexpected';

        const result = prepareError(err)!;

        expect(result).not.toContain('context:');
    });
});
