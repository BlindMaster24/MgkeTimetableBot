import { DayIndex } from '../../utils';
import type { RaspCache } from '../parser/raspCache';
import type { ArchiveAppendDay } from './repository';

export type SyncPlan = {
    entries: ArchiveAppendDay[];
    cacheMaxDay: number;
};

export function computeSyncPlan(raspCache: RaspCache): SyncPlan {
    const entries: ArchiveAppendDay[] = [];

    for (const [group, cacheEntry] of Object.entries(raspCache.groups.timetable)) {
        for (const day of cacheEntry.days) {
            entries.push({ type: 'group', value: group, day });
        }
    }

    for (const [teacher, cacheEntry] of Object.entries(raspCache.teachers.timetable)) {
        for (const day of cacheEntry.days) {
            entries.push({ type: 'teacher', value: teacher, day });
        }
    }

    const cacheMaxDay = entries.reduce((max, entry) => {
        const dayIndex = DayIndex.fromStringDate(entry.day.day).valueOf();
        return dayIndex > max ? dayIndex : max;
    }, 0);

    return { entries, cacheMaxDay };
}

export function shouldSync(cacheMaxDay: number, dbMaxDay: number): boolean {
    return cacheMaxDay > dbMaxDay;
}
