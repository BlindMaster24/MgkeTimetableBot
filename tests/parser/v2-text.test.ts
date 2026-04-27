import { describe, it, expect } from 'vitest';
import {
    cleanText,
    normalizeDate,
    parseDayLabel,
    parseLessonNumber,
    isTeacherLine
} from '../../src/services/parser/v2/text';

describe('cleanText', () => {
    it('collapses whitespace and trims', () => {
        expect(cleanText('  hello   world  ')).toBe('hello world');
    });

    it('returns null for null/undefined and empty-after-trim', () => {
        expect(cleanText(null)).toBeNull();
        expect(cleanText(undefined)).toBeNull();
        expect(cleanText('   ')).toBeNull();
    });
});

describe('normalizeDate', () => {
    it('pads single-digit day/month and expands 2-digit year to 20YY', () => {
        expect(normalizeDate('7.3.26')).toBe('07.03.2026');
    });

    it('keeps already-normalized values', () => {
        expect(normalizeDate('07.03.2026')).toBe('07.03.2026');
    });

    it('returns null for malformed input', () => {
        expect(normalizeDate('nope')).toBeNull();
        expect(normalizeDate('07.03')).toBeNull();
        expect(normalizeDate('0.0.0000')).toBeNull();
    });
});

describe('parseDayLabel', () => {
    it('extracts date and weekday from a mixed-case label', () => {
        const res = parseDayLabel('ПОНЕДЕЛЬНИК 02.03.2026');
        expect(res).toEqual({ day: '02.03.2026', weekday: 'Понедельник' });
    });

    it('returns null when no date is present', () => {
        expect(parseDayLabel('Понедельник')).toBeNull();
    });

    it('returns only the day when weekday is missing', () => {
        expect(parseDayLabel('02.03.2026')).toEqual({ day: '02.03.2026', weekday: undefined });
    });
});

describe('parseLessonNumber', () => {
    it('extracts the first contiguous digits', () => {
        expect(parseLessonNumber('Пара 3')).toBe(3);
        expect(parseLessonNumber('12. Лекция')).toBe(12);
    });

    it('returns null when there are no digits', () => {
        expect(parseLessonNumber('Лекция')).toBeNull();
        expect(parseLessonNumber(null)).toBeNull();
    });
});

describe('isTeacherLine', () => {
    it('matches "Surname I.I." pattern with optional second initial', () => {
        expect(isTeacherLine('Иванов И.И.')).toBe(true);
        expect(isTeacherLine('Петров П.')).toBe(true);
    });

    it('rejects non-teacher strings', () => {
        expect(isTeacherLine('math')).toBe(false);
        expect(isTeacherLine('Кабинет 305')).toBe(false);
        expect(isTeacherLine('')).toBe(false);
    });
});
