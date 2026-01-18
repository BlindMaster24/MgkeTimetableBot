import { StringDate } from "../../../utils";
import { Group, Groups, Teacher, Teachers } from "../types";

type ValidationResult = {
    ok: boolean,
    errors: string[]
}

function shuffle<T>(items: T[]): T[] {
    const copy = items.slice();
    for (let i = copy.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        const tmp = copy[i];
        copy[i] = copy[j];
        copy[j] = tmp;
    }
    return copy;
}

function pickSample<T extends Group | Teacher>(entries: T[], sampleSize: number): T[] {
    if (sampleSize <= 0 || entries.length <= sampleSize) {
        return entries;
    }

    const withLessons = entries.filter((entry) => {
        return entry.days.some((day) => day.lessons.length > 0);
    });

    const source = withLessons.length > 0 ? withLessons : entries;
    return shuffle(source).slice(0, sampleSize);
}

function validateEntry(entry: Group | Teacher, maxLessonsPerDay: number): string[] {
    const errors: string[] = [];

    if (!entry.days || entry.days.length === 0) {
        errors.push('no days');
        return errors;
    }

    const seen = new Set<string>();

    for (const day of entry.days) {
        if (!day.day) {
            errors.push('empty day');
            continue;
        }

        if (seen.has(day.day)) {
            errors.push(`duplicate day ${day.day}`);
        }

        seen.add(day.day);

        try {
            StringDate.fromStringDate(day.day);
        } catch {
            errors.push(`invalid date ${day.day}`);
        }

        if (day.lessons.length > maxLessonsPerDay) {
            errors.push(`too many lessons ${day.day}`);
        }
    }

    return errors;
}

export function validateGroups(groups: Groups, maxLessonsPerDay: number, sampleSize: number): ValidationResult {
    const errors: string[] = [];
    const entries = Object.values(groups);

    if (entries.length === 0) {
        return { ok: false, errors: ['empty groups'] };
    }

    let hasLessons = false;
    for (const entry of pickSample(entries, sampleSize)) {
        const entryErrors = validateEntry(entry, maxLessonsPerDay);
        if (entryErrors.length > 0) {
            errors.push(`${entry.group}: ${entryErrors.join(', ')}`);
        }
        if (!hasLessons) {
            hasLessons = entry.days.some((day) => day.lessons.length > 0);
        }
    }

    if (!hasLessons) {
        errors.push('no lessons in sample');
    }

    return {
        ok: errors.length === 0,
        errors
    };
}

export function validateTeachers(teachers: Teachers, maxLessonsPerDay: number, sampleSize: number): ValidationResult {
    const errors: string[] = [];
    const entries = Object.values(teachers);

    if (entries.length === 0) {
        return { ok: false, errors: ['empty teachers'] };
    }

    let hasLessons = false;
    for (const entry of pickSample(entries, sampleSize)) {
        const entryErrors = validateEntry(entry, maxLessonsPerDay);
        if (entryErrors.length > 0) {
            errors.push(`${entry.teacher}: ${entryErrors.join(', ')}`);
        }
        if (!hasLessons) {
            hasLessons = entry.days.some((day) => day.lessons.length > 0);
        }
    }

    if (!hasLessons) {
        errors.push('no lessons in sample');
    }

    return {
        ok: errors.length === 0,
        errors
    };
}
