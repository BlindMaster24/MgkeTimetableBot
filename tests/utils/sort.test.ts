import { describe, it, expect } from 'vitest';
import { sort } from '../../src/utils/sort';

describe('sort', () => {
    it('sorts numeric-like strings numerically', () => {
        expect(sort(['10', '2', '1', '20'])).toEqual([1, 2, 10, 20]);
    });

    it('orders numbers before strings', () => {
        expect(sort(['b', '1', 'a', '2'])).toEqual([1, 2, 'a', 'b']);
    });

    it('does not mutate the input array', () => {
        const input = [3, 1, 2];
        const output = sort(input);

        expect(input).toEqual([3, 1, 2]);
        expect(output).toEqual([1, 2, 3]);
    });

    it('keeps pure string arrays in lexical order', () => {
        expect(sort(['b', 'a', 'c'])).toEqual(['a', 'b', 'c']);
    });
});
