import { describe, it, expect } from 'vitest';
import { validateGroups, validateTeachers } from '../../src/services/parser/v2/validate';

describe('parser v2 validate', () => {
    it('flags empty groups and empty teachers', () => {
        const groups = validateGroups({}, 6, 0);
        expect(groups.ok).toBe(false);
        expect(groups.errors).toContain('empty groups');

        const teachers = validateTeachers({}, 6, 0);
        expect(teachers.ok).toBe(false);
        expect(teachers.errors).toContain('empty teachers');
    });

    it('flags duplicate days and too-many-lessons in group entries', () => {
        const groups: any = {
            '167': {
                group: '167',
                days: [
                    { day: 'bad-date', lessons: [{ title: 'A' }, { title: 'B' }] },
                    { day: 'bad-date', lessons: [{ title: 'C' }, { title: 'D' }] }
                ]
            }
        };

        const out = validateGroups(groups, 1, 0);
        const errors = out.errors.join(' ');
        expect(out.ok).toBe(false);
        expect(errors).toMatch(/duplicate day/);
        expect(errors).toMatch(/too many lessons/);
    });

    it('flags a teacher whose sample day has no lessons', () => {
        const teachers: any = {
            'Ivanov I.I.': {
                teacher: 'Ivanov I.I.',
                days: [{ day: '01.02.2026', lessons: [] }]
            }
        };

        const out = validateTeachers(teachers, 6, 0);
        expect(out.ok).toBe(false);
        expect(out.errors).toContain('no lessons in sample');
    });
});
