import { describe, it, expect, vi, afterEach } from 'vitest';
import { formatSeconds, nowInTime, seconds2times } from '../../src/utils/time';

describe('seconds2times', () => {
    it('splits 3661 seconds into [1, 1, 1, 0] (sec, min, h, d, y placeholders)', () => {
        const t = seconds2times(3661);
        expect(t[0]).toBe(1);
        expect(t[1]).toBe(1);
        expect(t[2]).toBe(1);
    });

    it('returns [seconds] for values under a minute', () => {
        expect(seconds2times(42)[0]).toBe(42);
    });
});

describe('formatSeconds', () => {
    it('formats to Russian labels in descending order', () => {
        expect(formatSeconds(3661)).toBe('1 ч. 1 мин. 1 сек.');
    });

    it('respects the limit parameter', () => {
        expect(formatSeconds(3661, 2)).toBe('1 ч. 1 мин.');
        expect(formatSeconds(3661, 1)).toBe('1 ч.');
    });

    it('formats pure-second values without other units', () => {
        expect(formatSeconds(42)).toBe('42 сек.');
    });
});

describe('nowInTime', () => {
    afterEach(() => vi.useRealTimers());

    it('returns true inside the window on an included weekday', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date(2026, 2, 4, 12, 30));

        expect(nowInTime([3], '10:00', '14:00')).toBe(true);
    });

    it('returns false when the weekday is not included', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date(2026, 2, 4, 12, 30));

        expect(nowInTime([1, 2], '10:00', '14:00')).toBe(false);
    });

    it('returns false when current time is outside the window', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date(2026, 2, 4, 9, 30));

        expect(nowInTime([3], '10:00', '14:00')).toBe(false);
    });
});
