import { describe, it, expect } from 'vitest';
import { randArray } from '../../src/utils/rand';

describe('randArray', () => {
    it('always returns an element from the given array', () => {
        const values = ['a', 'b', 'c', 'd', 'e'];

        for (let i = 0; i < 100; i++) {
            expect(values).toContain(randArray(values));
        }
    });

    it('returns the single element when the array has length 1', () => {
        expect(randArray(['only'])).toBe('only');
    });
});
