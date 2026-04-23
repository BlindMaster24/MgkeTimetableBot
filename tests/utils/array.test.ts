import { describe, it, expect } from 'vitest';
import { arrayUnique, chunkArray } from '../../src/utils/array';

describe('arrayUnique', () => {
    it('removes duplicates while preserving insertion order', () => {
        expect(arrayUnique([1, 2, 2, 3, 1, 4])).toEqual([1, 2, 3, 4]);
        expect(arrayUnique(['a', 'b', 'a'])).toEqual(['a', 'b']);
    });

    it('returns an empty array for empty input', () => {
        expect(arrayUnique([])).toEqual([]);
    });
});

describe('chunkArray', () => {
    it('splits an array into fixed-size chunks', () => {
        expect(chunkArray([1, 2, 3, 4, 5, 6], 2)).toEqual([
            [1, 2],
            [3, 4],
            [5, 6]
        ]);
    });

    it('pads the last chunk with undefined when not divisible', () => {
        expect(chunkArray([1, 2, 3, 4, 5], 2)).toEqual([
            [1, 2],
            [3, 4],
            [5, undefined]
        ]);
    });

    it('returns a single chunk when size exceeds length', () => {
        expect(chunkArray([1, 2], 5)).toEqual([[1, 2, undefined, undefined, undefined]]);
    });

    it('returns [] for an empty input regardless of size', () => {
        expect(chunkArray([], 3)).toEqual([]);
    });
});
