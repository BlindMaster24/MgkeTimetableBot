import { describe, it, expect } from 'vitest';
import { mergeDays } from '../../src/utils/combineRasp';
import type { GroupDay } from '../../src/services/parser/types';

const day = (d: string, lessons: GroupDay['lessons'] = []): GroupDay => ({ day: d, lessons });

describe('mergeDays', () => {
    it('preserves untouched old days and reports nothing added or changed', () => {
        const oldDays = [day('01.03.2026'), day('02.03.2026')];
        const newDays: GroupDay[] = [];

        const res = mergeDays(newDays, oldDays);

        expect(res.mergedDays).toHaveLength(2);
        expect(res.added).toEqual([]);
        expect(res.changed).toEqual([]);
    });

    it('marks a new day as added and keeps previous days', () => {
        const oldDays = [day('01.03.2026')];
        const newDays = [day('02.03.2026')];

        const res = mergeDays(newDays, oldDays);

        expect(res.added).toEqual([day('02.03.2026')]);
        expect(res.changed).toEqual([]);
        expect(res.mergedDays.map(d => d.day).sort()).toEqual(['01.03.2026', '02.03.2026']);
    });

    it('marks a day with different lessons as changed and overwrites it in the merged list', () => {
        const oldDays = [day('01.03.2026', [{ lesson: 'math' } as any])];
        const newDays = [day('01.03.2026', [{ lesson: 'physics' } as any])];

        const res = mergeDays(newDays, oldDays);

        expect(res.added).toEqual([]);
        expect(res.changed).toEqual(newDays);
        expect(res.mergedDays).toEqual(newDays);
    });

    it('does not flag an identical day as changed', () => {
        const lessons = [{ lesson: 'x' } as any];
        const oldDays = [day('01.03.2026', lessons)];
        const newDays = [day('01.03.2026', lessons)];

        const res = mergeDays(newDays, oldDays);

        expect(res.changed).toEqual([]);
        expect(res.added).toEqual([]);
    });
});
