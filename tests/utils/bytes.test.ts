import { describe, it, expect } from 'vitest';
import { formatBytes } from '../../src/utils/bytes';

describe('formatBytes', () => {
    it('returns "0 Bytes" for zero', () => {
        expect(formatBytes(0)).toBe('0 Bytes');
    });

    it('formats kilobytes, megabytes, gigabytes with default 2 decimals', () => {
        expect(formatBytes(1024)).toBe('1 KB');
        expect(formatBytes(1024 * 1024)).toBe('1 MB');
        expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GB');
    });

    it('rounds fractional values to requested decimals', () => {
        expect(formatBytes(1536, 0)).toBe('2 KB');
        expect(formatBytes(1536, 1)).toBe('1.5 KB');
        expect(formatBytes(1536, 2)).toBe('1.5 KB');
    });

    it('treats negative decimals as zero decimals', () => {
        expect(formatBytes(1536, -5)).toBe('2 KB');
    });
});
