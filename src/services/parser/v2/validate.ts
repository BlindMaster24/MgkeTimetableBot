import { StringDate } from "../../../utils";
import { Group, Groups, Teacher, Teachers } from "../types";

type ValidationResult = {
    ok: boolean,
    errors: string[]
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

export function validateGroups(groups: Groups, maxLessonsPerDay: number): ValidationResult {
    const errors: string[] = [];
    const entries = Object.values(groups);

    if (entries.length === 0) {
        return { ok: false, errors: ['empty groups'] };
    }

    for (const entry of entries) {
        const entryErrors = validateEntry(entry, maxLessonsPerDay);
        if (entryErrors.length > 0) {
            errors.push(`${entry.group}: ${entryErrors.join(', ')}`);
        }
    }

    return {
        ok: errors.length === 0,
        errors
    };
}

export function validateTeachers(teachers: Teachers, maxLessonsPerDay: number): ValidationResult {
    const errors: string[] = [];
    const entries = Object.values(teachers);

    if (entries.length === 0) {
        return { ok: false, errors: ['empty teachers'] };
    }

    for (const entry of entries) {
        const entryErrors = validateEntry(entry, maxLessonsPerDay);
        if (entryErrors.length > 0) {
            errors.push(`${entry.teacher}: ${entryErrors.join(', ')}`);
        }
    }

    return {
        ok: errors.length === 0,
        errors
    };
}
