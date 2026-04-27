import { describe, expect, it } from 'vitest';
import type { RaspCache } from '../../src/services/parser/raspCache';
import { computeSyncPlan, shouldSync } from '../../src/services/timetable/syncPlan';
import { DayIndex } from '../../src/utils';

const mkCache = (groups: Record<string, string[]>, teachers: Record<string, string[]>): RaspCache =>
    ({
        groups: {
            update: Date.now(),
            lastWeekIndex: 0,
            timetable: Object.fromEntries(
                Object.entries(groups).map(([group, dates]) => [
                    group,
                    { group, days: dates.map((d) => ({ day: d, lessons: [] })) }
                ])
            )
        },
        teachers: {
            update: Date.now(),
            lastWeekIndex: 0,
            timetable: Object.fromEntries(
                Object.entries(teachers).map(([teacher, dates]) => [
                    teacher,
                    { teacher, days: dates.map((d) => ({ day: d, lessons: [] })) }
                ])
            )
        },
        team: { update: 0, names: {} },
        successUpdate: true
    }) as unknown as RaspCache;

describe('computeSyncPlan', () => {
    it('returns no entries when cache is empty', () => {
        const plan = computeSyncPlan(mkCache({}, {}));
        expect(plan.entries).toEqual([]);
        expect(plan.cacheMaxDay).toBe(0);
    });

    it('collects one entry per group-day and teacher-day pair', () => {
        const plan = computeSyncPlan(mkCache({ 'ПС-11': ['01.09.2024', '02.09.2024'] }, { Ivanov: ['01.09.2024'] }));

        expect(plan.entries).toHaveLength(3);
        expect(plan.entries.filter((e) => e.type === 'group')).toHaveLength(2);
        expect(plan.entries.filter((e) => e.type === 'teacher')).toHaveLength(1);
    });

    it('computes cacheMaxDay as the maximum DayIndex across all entries', () => {
        const plan = computeSyncPlan(mkCache({ A: ['01.09.2024', '05.09.2024', '03.09.2024'] }, {}));
        const expected = DayIndex.fromStringDate('05.09.2024').valueOf();
        expect(plan.cacheMaxDay).toBe(expected);
    });

    it('handles caches with only teachers', () => {
        const plan = computeSyncPlan(mkCache({}, { Sidorov: ['10.09.2024'] }));
        expect(plan.entries).toHaveLength(1);
        expect(plan.entries[0].type).toBe('teacher');
        expect(plan.cacheMaxDay).toBe(DayIndex.fromStringDate('10.09.2024').valueOf());
    });
});

describe('shouldSync', () => {
    it('returns true when cacheMaxDay is strictly greater than dbMaxDay', () => {
        expect(shouldSync(100, 99)).toBe(true);
    });

    it('returns false when cacheMaxDay is equal to dbMaxDay', () => {
        expect(shouldSync(100, 100)).toBe(false);
    });

    it('returns false when cacheMaxDay is less than dbMaxDay', () => {
        expect(shouldSync(50, 100)).toBe(false);
    });

    it('treats missing dbMaxDay (0) as a fresh DB that needs full sync', () => {
        expect(shouldSync(100, 0)).toBe(true);
    });

    it('returns false on an empty cache (cacheMaxDay=0) against a fresh DB', () => {
        expect(shouldSync(0, 0)).toBe(false);
    });
});
