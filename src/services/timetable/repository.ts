import { Op } from 'sequelize';
import { sequelize } from '../../db';
import { DayIndex, StringDate } from '../../utils';
import { GroupDay, TeacherDay } from '../parser/types';
import { TimetableArchive } from './models/timetable';

type TimetableArchiveDay = GroupDay | TeacherDay;

export type ArchiveAppendDay =
    | {
          type: 'group';
          value: string;
          day: GroupDay;
      }
    | {
          type: 'teacher';
          value: string;
          day: TeacherDay;
      };

type ArchiveBounds = {
    min: number;
    max: number;
};

function dbEntryToDayObject(entry: TimetableArchive): TimetableArchiveDay {
    return {
        day: StringDate.fromDayIndex(entry.day).toString(),
        lessons: JSON.parse(entry.data)
    };
}

export class TimetableArchiveRepository {
    public async getDayIndexBounds(): Promise<ArchiveBounds> {
        const { fn, col } = sequelize;

        const data = await TimetableArchive.findOne({
            attributes: [
                [fn('min', col('day')), 'min'],
                [fn('max', col('day')), 'max']
            ],
            rejectOnEmpty: true
        });

        return {
            min: data.get('min') as number,
            max: data.get('max') as number
        };
    }

    public async getGroups(): Promise<string[]> {
        const data = await TimetableArchive.findAll({
            attributes: ['group'],
            where: {
                group: {
                    [Op.not]: null
                }
            },
            group: 'group'
        });

        return data.map((entry) => entry.group!);
    }

    public async getTeachers(): Promise<string[]> {
        const data = await TimetableArchive.findAll({
            attributes: ['teacher'],
            where: {
                teacher: {
                    [Op.not]: null
                }
            },
            group: 'teacher'
        });

        return data.map((entry) => entry.teacher!);
    }

    public async getGroupDay(dayIndex: number, group: string): Promise<GroupDay | null> {
        const entry = await TimetableArchive.findOne({
            attributes: ['day', 'data'],
            where: {
                day: dayIndex,
                group: group
            }
        });

        return entry ? (dbEntryToDayObject(entry) as GroupDay) : null;
    }

    public async getTeacherDay(dayIndex: number, teacher: string): Promise<TeacherDay | null> {
        const entry = await TimetableArchive.findOne({
            attributes: ['day', 'data'],
            where: {
                day: dayIndex,
                teacher: teacher
            }
        });

        return entry ? (dbEntryToDayObject(entry) as TeacherDay) : null;
    }

    public async getGroupDaysByRange(dayBounds: [number, number], group: string): Promise<GroupDay[]> {
        const days = await TimetableArchive.findAll({
            attributes: ['day', 'data'],
            where: {
                group: group,
                day: {
                    [Op.between]: dayBounds
                }
            },
            order: [['day', 'ASC']]
        });

        return days.map((entry) => dbEntryToDayObject(entry) as GroupDay);
    }

    public async getTeacherDaysByRange(dayBounds: [number, number], teacher: string): Promise<TeacherDay[]> {
        const days = await TimetableArchive.findAll({
            attributes: ['day', 'data'],
            where: {
                teacher: teacher,
                day: {
                    [Op.between]: dayBounds
                }
            },
            order: [['day', 'ASC']]
        });

        return days.map((entry) => dbEntryToDayObject(entry) as TeacherDay);
    }

    public async getGroupDays(group: string, fromDay?: number): Promise<GroupDay[]> {
        const days = await TimetableArchive.findAll({
            attributes: ['day', 'data'],
            where: {
                group: group,
                ...(fromDay !== undefined
                    ? {
                          day: {
                              [Op.gte]: fromDay
                          }
                      }
                    : {})
            },
            order: [['day', 'ASC']]
        });

        return days.map((entry) => dbEntryToDayObject(entry) as GroupDay);
    }

    public async getTeacherDays(teacher: string, fromDay?: number): Promise<TeacherDay[]> {
        const days = await TimetableArchive.findAll({
            attributes: ['day', 'data'],
            where: {
                teacher: teacher,
                ...(fromDay !== undefined
                    ? {
                          day: {
                              [Op.gte]: fromDay
                          }
                      }
                    : {})
            },
            order: [['day', 'ASC']]
        });

        return days.map((entry) => dbEntryToDayObject(entry) as TeacherDay);
    }

    public toArchiveRow(entry: ArchiveAppendDay): { day: number; group?: string; teacher?: string; data: string } {
        const dayIndex: number = DayIndex.fromStringDate(entry.day.day).valueOf();
        const data: string = JSON.stringify(entry.day.lessons);

        if (entry.type === 'group') {
            return {
                day: dayIndex,
                group: entry.value,
                data: data
            };
        }

        return {
            day: dayIndex,
            teacher: entry.value,
            data: data
        };
    }
}
