import { describe, it, expect } from 'vitest';
import { DayIndex, StringDate } from '../../../src/utils/time';

describe('StringDate', () => {
    it('fromStringDate parses DD.MM.YYYY in local time', () => {
        const sd = StringDate.fromStringDate('07.03.2026');
        const date = sd.toDate();

        expect(date.getFullYear()).toBe(2026);
        expect(date.getMonth()).toBe(2);
        expect(date.getDate()).toBe(7);
        expect(date.getHours()).toBe(0);
        expect(date.getMinutes()).toBe(0);
    });

    it('fromStringDate with utc=true sets UTC midnight', () => {
        const sd = StringDate.fromStringDate('07.03.2026', true);
        const date = sd.toDate();

        expect(date.getUTCFullYear()).toBe(2026);
        expect(date.getUTCMonth()).toBe(2);
        expect(date.getUTCDate()).toBe(7);
        expect(date.getUTCHours()).toBe(0);
    });

    it('fromStringDateTime combines date and time parts', () => {
        const sd = StringDate.fromStringDateTime('07.03.2026', '14:30');
        const date = sd.toDate();

        expect(date.getFullYear()).toBe(2026);
        expect(date.getMonth()).toBe(2);
        expect(date.getDate()).toBe(7);
        expect(date.getHours()).toBe(14);
        expect(date.getMinutes()).toBe(30);
    });

    it('toStringDate pads day and month with zeros', () => {
        const sd = StringDate.fromStringDate('01.02.2026');
        expect(sd.toStringDate()).toBe('01.02.2026');
        expect(sd.toStringDateNoYear()).toBe('01.02');
    });

    it('toStringTime uses HH:MM:SS and optionally milliseconds', () => {
        const sd = StringDate.fromStringDateTime('07.03.2026', '09:05');
        const time = sd.toStringTime();
        const timeWithMs = sd.toStringTime(true);

        expect(time).toBe('09:05:00');
        expect(timeWithMs).toBe('09:05:00,000');
    });

    it('toStringDateTime concatenates date and time', () => {
        const sd = StringDate.fromStringDateTime('07.03.2026', '09:05');
        expect(sd.toStringDateTime()).toBe('07.03.2026 09:05:00');
    });

    it('weekday predicates match the Russian weekday name', () => {
        const saturday = StringDate.fromStringDate('07.03.2026');
        const sunday = StringDate.fromStringDate('08.03.2026');

        expect(saturday.getWeekdayName()).toBe('Суббота');
        expect(saturday.isSaturday()).toBe(true);
        expect(saturday.isSunday()).toBe(false);

        expect(sunday.getWeekdayName()).toBe('Воскресенье');
        expect(sunday.isSunday()).toBe(true);
        expect(sunday.isSaturday()).toBe(false);
    });

    it('fromDayIndex accepts both number and DayIndex', () => {
        const fromNumber = StringDate.fromDayIndex(20500);
        const fromIdx = StringDate.fromDayIndex(DayIndex.fromNumber(20500));

        expect(fromNumber.valueOf()).toBe(fromIdx.valueOf());
    });

    it('fromUnixTime accepts both number and bigint', () => {
        const ts = Date.UTC(2026, 2, 7, 12, 0, 0);
        const a = StringDate.fromUnixTime(ts);
        const b = StringDate.fromUnixTime(BigInt(ts));

        expect(a.valueOf()).toBe(ts);
        expect(b.valueOf()).toBe(ts);
    });
});
