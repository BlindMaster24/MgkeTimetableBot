import { describe, it, expect, vi, afterEach } from 'vitest';
import { DayIndex, StringDate } from '../../../src/utils/time';

describe('DayIndex', () => {
    afterEach(() => {
        vi.useRealTimers();
    });

    it('fromStringDate accepts both string and StringDate and yields the same index', () => {
        const a = DayIndex.fromStringDate('15.09.2025');
        const b = DayIndex.fromStringDate(StringDate.fromStringDate('15.09.2025'));

        expect(a.valueOf()).toBe(b.valueOf());
    });

    it('fromNumber round-trips through valueOf and toString', () => {
        const idx = DayIndex.fromNumber(20500);

        expect(idx.valueOf()).toBe(20500);
        expect(idx.toString()).toBe('20500');
    });

    it('toDate yields midnight UTC that matches the original day index', () => {
        const original = DayIndex.fromStringDate('01.01.2025');
        const date = original.toDate();

        expect(date.getUTCHours()).toBe(0);
        expect(date.getUTCMinutes()).toBe(0);
        expect(date.getUTCSeconds()).toBe(0);
        expect(date.getUTCMilliseconds()).toBe(0);
    });

    it('isToday / isTomorrow / isFuture / isPast / isNotPast match a mocked clock', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date('2025-09-15T12:00:00Z'));

        const today = DayIndex.fromStringDate('15.09.2025');
        const tomorrow = DayIndex.fromStringDate('16.09.2025');
        const yesterday = DayIndex.fromStringDate('14.09.2025');

        expect(today.isToday()).toBe(true);
        expect(today.isTomorrow()).toBe(false);
        expect(today.isPast()).toBe(false);
        expect(today.isFuture()).toBe(false);
        expect(today.isNotPast()).toBe(true);

        expect(tomorrow.isToday()).toBe(false);
        expect(tomorrow.isTomorrow()).toBe(true);
        expect(tomorrow.isFuture()).toBe(true);
        expect(tomorrow.isNotPast()).toBe(true);

        expect(yesterday.isPast()).toBe(true);
        expect(yesterday.isNotPast()).toBe(false);
        expect(yesterday.isToday()).toBe(false);
        expect(yesterday.isFuture()).toBe(false);
    });

    it('DayIndex.now() equals fromDate(new Date()) at a fixed clock', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date('2025-09-15T10:30:00Z'));

        expect(DayIndex.now().valueOf()).toBe(DayIndex.fromDate(new Date()).valueOf());
    });
});
