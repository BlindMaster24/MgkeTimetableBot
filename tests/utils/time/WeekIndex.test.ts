import { describe, it, expect, vi, afterEach } from 'vitest';
import { DayIndex, StringDate, WeekIndex } from '../../../src/utils/time';

describe('WeekIndex', () => {
    afterEach(() => {
        vi.useRealTimers();
    });

    it('fromDate maps Monday..Saturday to the same week index; Sunday rolls back to the previous week', () => {
        const monday = WeekIndex.fromDate(new Date(2026, 2, 2)).valueOf();
        const tuesday = WeekIndex.fromDate(new Date(2026, 2, 3)).valueOf();
        const saturday = WeekIndex.fromDate(new Date(2026, 2, 7)).valueOf();
        const sunday = WeekIndex.fromDate(new Date(2026, 2, 8)).valueOf();
        const nextMonday = WeekIndex.fromDate(new Date(2026, 2, 9)).valueOf();

        expect(tuesday).toBe(monday);
        expect(saturday).toBe(monday);
        expect(sunday).toBe(monday);
        expect(nextMonday).toBe(monday + 1);
    });

    it('fromStringDate and fromDayIndex agree with fromDate', () => {
        const date = new Date(2026, 2, 4);
        const fromD = WeekIndex.fromDate(date).valueOf();
        const fromS = WeekIndex.fromStringDate('04.03.2026').valueOf();
        const fromI = WeekIndex.fromDayIndex(DayIndex.fromStringDate('04.03.2026')).valueOf();
        const fromINum = WeekIndex.fromDayIndex(DayIndex.fromStringDate('04.03.2026').valueOf()).valueOf();

        expect(fromS).toBe(fromD);
        expect(fromI).toBe(fromD);
        expect(fromINum).toBe(fromD);
    });

    it('getFirstDayDate returns the Monday of that week and getWeekRange covers 6 following days', () => {
        const wi = WeekIndex.fromStringDate('04.03.2026');
        const first = wi.getFirstDayDate();
        const [start, end] = wi.getWeekRange();

        expect(first.getDay()).toBe(1);
        expect(start.getTime()).toBe(first.getTime());
        expect(end.getTime() - start.getTime()).toBe(6 * 24 * 60 * 60 * 1000);
    });

    it('getNextWeekIndex increments by one', () => {
        const wi = WeekIndex.fromStringDate('04.03.2026');
        expect(wi.getNextWeekIndex().valueOf()).toBe(wi.valueOf() + 1);
    });

    it('isFutureWeek uses the current clock', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date('2026-03-04T12:00:00Z'));

        const current = WeekIndex.now();
        expect(current.isFutureWeek()).toBe(false);
        expect(current.getNextWeekIndex().isFutureWeek()).toBe(true);
    });

    it('getAcademicYearStartDate snaps to the first Monday on/after September 1', () => {
        const start2025 = WeekIndex.getAcademicYearStartDate(new Date(2025, 10, 10));
        const start2026 = WeekIndex.getAcademicYearStartDate(new Date(2026, 10, 10));

        expect(start2025.getDay()).toBe(1);
        expect(start2026.getDay()).toBe(1);
        expect(start2025.getFullYear()).toBe(2025);
        expect(start2026.getFullYear()).toBe(2026);
    });

    it('getAcademicWeekNumber returns 1 at the start and grows linearly', () => {
        const start = WeekIndex.getAcademicYearStartDate(new Date(2025, 10, 10));
        const plusOne = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000);

        expect(WeekIndex.getAcademicWeekNumber(start)).toBe(1);
        expect(WeekIndex.getAcademicWeekNumber(plusOne)).toBe(2);
    });

    it('fromAcademicWeekNumber is the inverse of getAcademicWeekNumber for integer week boundaries', () => {
        const base = new Date(2025, 10, 10);
        const weekNo = WeekIndex.getAcademicWeekNumber(base);
        const rebuilt = WeekIndex.fromAcademicWeekNumber(weekNo, base);

        expect(rebuilt.getAcademicWeekNumber()).toBe(weekNo);
    });

    it('fromWeekIndexNumber round-trips through valueOf and toString', () => {
        const wi = WeekIndex.fromWeekIndexNumber(2945);
        expect(wi.valueOf()).toBe(2945);
        expect(wi.toString()).toBe('2945');
    });

    it('getWeekDayIndexRange returns DayIndex values spanning 6 days', () => {
        const wi = WeekIndex.fromStringDate('04.03.2026');
        const [from, to] = wi.getWeekDayIndexRange();

        expect(to - from).toBe(6);
    });

    it('WeekIndex.now() agrees with fromDate(new Date()) at a fixed clock', () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date('2026-03-04T12:00:00Z'));

        expect(WeekIndex.now().valueOf()).toBe(WeekIndex.fromDate(new Date()).valueOf());
    });

    it('ignores StringDate input via fromStringDate shortcut', () => {
        const sd = StringDate.fromStringDate('04.03.2026');
        expect(WeekIndex.fromStringDate(sd).valueOf()).toBe(WeekIndex.fromDate(sd.toDate()).valueOf());
    });
});
