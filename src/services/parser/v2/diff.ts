import { Group, Groups, Teacher, Teachers } from "../types";

type DiffLine = {
    key: string,
    day: string,
    reason: string
}

function diffEntries(newEntry: Group | Teacher, oldEntry?: Group | Teacher): DiffLine[] {
    const lines: DiffLine[] = [];
    const oldMap = new Map<string, Group['days'][number] | Teacher['days'][number]>();

    if (oldEntry) {
        for (const day of oldEntry.days) {
            oldMap.set(day.day, day);
        }
    }

    for (const day of newEntry.days) {
        const oldDay = oldMap.get(day.day);
        if (!oldDay) {
            lines.push({ key: 'new', day: day.day, reason: 'added' });
            continue;
        }

        if (JSON.stringify(oldDay.lessons) !== JSON.stringify(day.lessons)) {
            lines.push({ key: 'changed', day: day.day, reason: 'updated' });
        }

        oldMap.delete(day.day);
    }

    for (const [day] of oldMap) {
        lines.push({ key: 'old', day: day, reason: 'removed' });
    }

    return lines;
}

export function diffGroups(current: Groups, previous: Groups, limit: number): string[] {
    const output: string[] = [];
    const keys = new Set<string>([...Object.keys(current), ...Object.keys(previous)]);

    for (const key of keys) {
        const lines = diffEntries(current[key], previous[key]);
        for (const line of lines) {
            output.push(`${key}: ${line.reason} ${line.day}`);
            if (output.length >= limit) {
                return output;
            }
        }
    }

    return output;
}

export function diffTeachers(current: Teachers, previous: Teachers, limit: number): string[] {
    const output: string[] = [];
    const keys = new Set<string>([...Object.keys(current), ...Object.keys(previous)]);

    for (const key of keys) {
        const lines = diffEntries(current[key], previous[key]);
        for (const line of lines) {
            output.push(`${key}: ${line.reason} ${line.day}`);
            if (output.length >= limit) {
                return output;
            }
        }
    }

    return output;
}
