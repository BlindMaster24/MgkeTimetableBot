import { App, AppService } from '../../app';
import { sequelize } from '../../db';
import { Logger } from '../../logger';
import { DayIndex, WeekIndex } from '../../utils';
import { loadCache, raspCache } from '../parser/raspCache';
import { GroupDay, TeacherDay } from '../parser/types';
import { TimetableArchive } from './models/timetable';
import { ArchiveAppendDay, TimetableArchiveRepository } from './repository';

export class Timetable implements AppService {
    private readonly repository = new TimetableArchiveRepository();
    private readonly logger = new Logger('Timetable');

    constructor(private app: App) {}

    public run() {
        if (this.app.isServiceRegistered('parser')) {
            const parser = this.app.getService('parser');

            parser.events.on('flushCache', this.appendDays.bind(this));
        }

        loadCache();

        this.syncFromCacheIfStale().catch((error) => {
            this.logger.warn('syncFromCacheIfStale failed', error as Error);
        });
    }

    private async syncFromCacheIfStale(): Promise<void> {
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

        if (entries.length === 0) {
            return;
        }

        const cacheMaxDay = entries.reduce((max, entry) => {
            const dayIndex = DayIndex.fromStringDate(entry.day.day).valueOf();
            return dayIndex > max ? dayIndex : max;
        }, 0);

        let dbMaxDay = 0;
        try {
            const bounds = await this.repository.getDayIndexBounds();
            dbMaxDay = bounds.max ?? 0;
        } catch {
            dbMaxDay = 0;
        }

        if (cacheMaxDay <= dbMaxDay) {
            return;
        }

        this.logger.info('syncing cache to archive', { entries: entries.length, cacheMaxDay, dbMaxDay });
        await this.appendDays(entries);
    }

    public async getDayIndexBounds(): Promise<{ min: number; max: number }> {
        return this.repository.getDayIndexBounds();
    }

    public async getWeekIndexBounds(): Promise<{ min: number; max: number }> {
        const { min, max } = await this.getDayIndexBounds();

        return {
            min: WeekIndex.fromDayIndex(min).valueOf(),
            max: WeekIndex.fromDayIndex(max).valueOf()
        };
    }

    public async getGroups(): Promise<string[]> {
        return this.repository.getGroups();
    }

    public async getTeachers(): Promise<string[]> {
        return this.repository.getTeachers();
    }

    public async getGroupDay(dayIndex: number, group: string): Promise<GroupDay | null> {
        return this.repository.getGroupDay(dayIndex, group);
    }

    public async getTeacherDay(dayIndex: number, teacher: string): Promise<TeacherDay | null> {
        return this.repository.getTeacherDay(dayIndex, teacher);
    }

    public async getGroupDaysByRange(dayBounds: [number, number], group: string): Promise<GroupDay[]> {
        return this.repository.getGroupDaysByRange(dayBounds, group);
    }

    public async getTeacherDaysByRange(dayBounds: [number, number], teacher: string): Promise<TeacherDay[]> {
        return this.repository.getTeacherDaysByRange(dayBounds, teacher);
    }

    public async getGroupDays(group: string, fromDay?: number): Promise<GroupDay[]> {
        return this.repository.getGroupDays(group, fromDay);
    }

    public async getTeacherDays(teacher: string, fromDay?: number): Promise<TeacherDay[]> {
        return this.repository.getTeacherDays(teacher, fromDay);
    }

    public async appendDays(entries: ArchiveAppendDay[]) {
        if (entries.length === 0) {
            return;
        }

        await sequelize.transaction(async (transaction) => {
            await TimetableArchive.bulkCreate(
                entries
                    .filter((entry) => {
                        return entry.type === 'group';
                    })
                    .map((entry) => this.repository.toArchiveRow(entry)),
                {
                    transaction,
                    returning: false,
                    updateOnDuplicate: ['data'],
                    conflictAttributes: ['day', 'group']
                }
            );

            await TimetableArchive.bulkCreate(
                entries
                    .filter((entry) => {
                        return entry.type === 'teacher';
                    })
                    .map((entry) => this.repository.toArchiveRow(entry)),
                {
                    transaction,
                    returning: false,
                    updateOnDuplicate: ['data'],
                    conflictAttributes: ['day', 'teacher']
                }
            );
        });
    }
}

export * from '../parser/types';
export type { ArchiveAppendDay } from './repository';
