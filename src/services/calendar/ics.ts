import { createHash } from "crypto";
import { config } from "../../../config";
import { StringDate, WeekIndex } from "../../utils";
import { GroupDay, GroupLesson, GroupLessonExplain, TeacherDay, TeacherLessonExplain } from "../parser/types";

type CalendarType = 'group' | 'teacher';

type BuildIcsOptions = {
    type: CalendarType;
    value: string;
    weekIndex: WeekIndex;
    days: GroupDay[] | TeacherDay[];
};

type EventEntry = {
    uid: string;
    summary: string;
    description?: string;
    location?: string;
    start: Date;
    end: Date;
};

export function buildIcs(options: BuildIcsOptions): string {
    const events = buildEvents(options);
    const lines: string[] = [
        'BEGIN:VCALENDAR',
        'VERSION:2.0',
        'PRODID:-//MGKE Timetable Bot//EN',
        'CALSCALE:GREGORIAN',
        'METHOD:PUBLISH'
    ];

    for (const event of events) {
        lines.push('BEGIN:VEVENT');
        lines.push(`UID:${event.uid}`);
        lines.push(`DTSTAMP:${formatDateTime(new Date())}`);
        lines.push(`DTSTART:${formatDateTime(event.start)}`);
        lines.push(`DTEND:${formatDateTime(event.end)}`);
        lines.push(`SUMMARY:${escapeText(event.summary)}`);
        if (event.description) {
            lines.push(`DESCRIPTION:${escapeText(event.description)}`);
        }
        if (event.location) {
            lines.push(`LOCATION:${escapeText(event.location)}`);
        }
        lines.push('END:VEVENT');
    }

    lines.push('END:VCALENDAR');

    return lines.join('\r\n');
}

function buildEvents({ type, value, weekIndex, days }: BuildIcsOptions): EventEntry[] {
    const events: EventEntry[] = [];
    const weekNumber = weekIndex.getAcademicWeekNumber();

    for (const day of days) {
        const date = StringDate.fromStringDate(day.day);
        const isSaturday = date.isSaturday();
        const calls = config.timetable[isSaturday ? 'saturday' : 'weekdays'];

        for (let i = 0; i < day.lessons.length; i += 1) {
            const lesson = day.lessons[i] as GroupLesson | TeacherLessonExplain | null;
            const call = calls[i];
            if (!call) continue;

            const start = StringDate.fromStringDateTime(day.day, call[0][0]).toDate();
            const end = StringDate.fromStringDateTime(day.day, call[1][1]).toDate();

            if (Array.isArray(lesson)) {
                for (const sub of lesson) {
                    if (!sub || !sub.lesson) continue;
                    events.push(createEvent(type, value, weekNumber, day.day, i, start, end, sub));
                }
                continue;
            }

            if (!lesson || !lesson.lesson) {
                continue;
            }

            events.push(createEvent(type, value, weekNumber, day.day, i, start, end, lesson as GroupLessonExplain | TeacherLessonExplain));
        }
    }

    return events;
}

function createEvent(
    type: CalendarType,
    value: string,
    weekNumber: number,
    day: string,
    index: number,
    start: Date,
    end: Date,
    lesson: GroupLessonExplain | TeacherLessonExplain
): EventEntry {
    const subgroup = 'subgroup' in lesson && lesson.subgroup ? `, подгр. ${lesson.subgroup}` : '';
    const typeText = lesson.type ? ` (${lesson.type})` : '';
    const summary = `${lesson.lesson}${typeText}${subgroup}`;

    const descriptionParts: string[] = [];
    if ('teacher' in lesson && lesson.teacher) {
        descriptionParts.push(`Преподаватель: ${lesson.teacher}`);
    }
    if ('group' in lesson && lesson.group) {
        descriptionParts.push(`Группа: ${lesson.group}`);
    }
    if (lesson.cabinet) {
        descriptionParts.push(`Кабинет: ${lesson.cabinet}`);
    }

    const description = descriptionParts.length ? descriptionParts.join('\\n') : undefined;
    const location = lesson.cabinet ?? undefined;
    const uidSource = `${type}:${value}:${weekNumber}:${day}:${index}:${lesson.lesson}:${lesson.cabinet ?? ''}:${lesson.type ?? ''}:${(lesson as any).subgroup ?? ''}`;
    const uid = createHash('sha256').update(uidSource).digest('hex');

    return {
        uid,
        summary,
        description,
        location,
        start,
        end
    };
}

function formatDateTime(date: Date): string {
    const yyyy = date.getFullYear().toString().padStart(4, '0');
    const mm = (date.getMonth() + 1).toString().padStart(2, '0');
    const dd = date.getDate().toString().padStart(2, '0');
    const hh = date.getHours().toString().padStart(2, '0');
    const mi = date.getMinutes().toString().padStart(2, '0');
    const ss = date.getSeconds().toString().padStart(2, '0');
    return `${yyyy}${mm}${dd}T${hh}${mi}${ss}`;
}

function escapeText(value: string): string {
    return value.replace(/\\/g, '\\\\').replace(/\n/g, '\\n').replace(/,/g, '\\,').replace(/;/g, '\\;');
}
