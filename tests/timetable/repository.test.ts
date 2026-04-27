import { DataTypes, Sequelize } from 'sequelize';
import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { TimetableArchive } from '../../src/services/timetable/models/timetable';
import { TimetableArchiveRepository } from '../../src/services/timetable/repository';
import { DayIndex } from '../../src/utils';

const testSequelize = new Sequelize({ dialect: 'sqlite', storage: ':memory:', logging: false });

beforeAll(async () => {
    TimetableArchive.init(
        {
            id: { type: DataTypes.INTEGER, primaryKey: true, autoIncrement: true },
            day: { type: DataTypes.INTEGER, allowNull: false },
            group: { type: DataTypes.STRING, allowNull: true },
            teacher: { type: DataTypes.STRING, allowNull: true },
            data: { type: DataTypes.TEXT, allowNull: false }
        },
        {
            sequelize: testSequelize,
            tableName: 'timetable_archive',
            indexes: [
                { fields: ['day', 'group'], unique: true },
                { fields: ['day', 'teacher'], unique: true }
            ]
        }
    );

    await testSequelize.sync({ force: true });
});

afterAll(async () => {
    await testSequelize.close();
});

beforeEach(async () => {
    await TimetableArchive.destroy({ where: {}, truncate: true });
});

const d = (str: string) => DayIndex.fromStringDate(str).valueOf();
const seed = async (rows: Array<{ day: string; group?: string; teacher?: string; lessons?: unknown[] }>) => {
    await TimetableArchive.bulkCreate(
        rows.map((r) => ({
            day: d(r.day),
            group: r.group ?? null,
            teacher: r.teacher ?? null,
            data: JSON.stringify(r.lessons ?? [])
        }))
    );
};

describe('TimetableArchiveRepository (in-memory sqlite)', () => {
    const repo = new TimetableArchiveRepository();

    it('getDayIndexBounds returns min and max day indexes', async () => {
        await seed([
            { day: '01.09.2024', group: 'A', lessons: [] },
            { day: '03.09.2024', group: 'A', lessons: [] },
            { day: '05.09.2024', group: 'B', lessons: [] }
        ]);

        const bounds = await repo.getDayIndexBounds();
        expect(bounds.min).toBe(d('01.09.2024'));
        expect(bounds.max).toBe(d('05.09.2024'));
    });

    it('getGroups returns deduplicated non-null groups', async () => {
        await seed([
            { day: '01.09.2024', group: 'A' },
            { day: '02.09.2024', group: 'A' },
            { day: '01.09.2024', group: 'B' },
            { day: '03.09.2024', teacher: 'Ivanov' }
        ]);

        const groups = await repo.getGroups();
        expect(groups.sort()).toEqual(['A', 'B']);
    });

    it('getTeachers returns deduplicated non-null teachers', async () => {
        await seed([
            { day: '01.09.2024', group: 'A' },
            { day: '02.09.2024', teacher: 'Ivanov' },
            { day: '03.09.2024', teacher: 'Petrov' },
            { day: '04.09.2024', teacher: 'Ivanov' }
        ]);

        const teachers = await repo.getTeachers();
        expect(teachers.sort()).toEqual(['Ivanov', 'Petrov']);
    });

    it('getGroupDay returns the right row for (day, group) and null otherwise', async () => {
        await seed([{ day: '01.09.2024', group: 'A', lessons: [{ lesson: 'Math' }] }]);

        const hit = await repo.getGroupDay(d('01.09.2024'), 'A');
        expect(hit).not.toBeNull();
        expect(hit!.day).toBe('01.09.2024');
        expect(hit!.lessons).toEqual([{ lesson: 'Math' }]);

        const miss = await repo.getGroupDay(d('02.09.2024'), 'A');
        expect(miss).toBeNull();
    });

    it('getTeacherDay returns the right row for (day, teacher) and null otherwise', async () => {
        await seed([{ day: '01.09.2024', teacher: 'Ivanov', lessons: [{ lesson: 'Physics' }] }]);

        const hit = await repo.getTeacherDay(d('01.09.2024'), 'Ivanov');
        expect(hit).not.toBeNull();
        expect(hit!.lessons).toEqual([{ lesson: 'Physics' }]);

        const miss = await repo.getTeacherDay(d('01.09.2024'), 'Unknown');
        expect(miss).toBeNull();
    });

    it('getGroupDaysByRange returns rows within [from, to] ordered by day ASC', async () => {
        await seed([
            { day: '01.09.2024', group: 'A' },
            { day: '03.09.2024', group: 'A' },
            { day: '05.09.2024', group: 'A' },
            { day: '07.09.2024', group: 'A' }
        ]);

        const days = await repo.getGroupDaysByRange([d('02.09.2024'), d('06.09.2024')], 'A');
        expect(days.map((row) => row.day)).toEqual(['03.09.2024', '05.09.2024']);
    });

    it('getGroupDays with fromDay filters rows and returns them ordered', async () => {
        await seed([
            { day: '01.09.2024', group: 'A' },
            { day: '03.09.2024', group: 'A' },
            { day: '05.09.2024', group: 'A' }
        ]);

        const all = await repo.getGroupDays('A');
        expect(all.map((r) => r.day)).toEqual(['01.09.2024', '03.09.2024', '05.09.2024']);

        const fromMid = await repo.getGroupDays('A', d('03.09.2024'));
        expect(fromMid.map((r) => r.day)).toEqual(['03.09.2024', '05.09.2024']);
    });

    it('getTeacherDaysByRange returns rows within [from, to] ordered by day ASC', async () => {
        await seed([
            { day: '01.09.2024', teacher: 'T1' },
            { day: '04.09.2024', teacher: 'T1' },
            { day: '07.09.2024', teacher: 'T1' }
        ]);

        const days = await repo.getTeacherDaysByRange([d('02.09.2024'), d('06.09.2024')], 'T1');
        expect(days.map((row) => row.day)).toEqual(['04.09.2024']);
    });

    it('getTeacherDays with fromDay filters rows and returns them ordered', async () => {
        await seed([
            { day: '01.09.2024', teacher: 'T1' },
            { day: '03.09.2024', teacher: 'T1' },
            { day: '05.09.2024', teacher: 'T1' }
        ]);

        const all = await repo.getTeacherDays('T1');
        expect(all.map((r) => r.day)).toEqual(['01.09.2024', '03.09.2024', '05.09.2024']);

        const fromMid = await repo.getTeacherDays('T1', d('03.09.2024'));
        expect(fromMid.map((r) => r.day)).toEqual(['03.09.2024', '05.09.2024']);
    });
});
