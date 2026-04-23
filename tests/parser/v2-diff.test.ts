import { describe, it, expect } from 'vitest';
import { diffGroups, diffTeachers } from '../../src/services/parser/v2/diff';

describe('parser v2 diff', () => {
    it('reports added/updated/removed days for a group', () => {
        const previous: any = {
            '167': {
                group: '167',
                days: [
                    { day: '01.02.2026', lessons: [{ title: 'Math' }] },
                    { day: '02.02.2026', lessons: [{ title: 'Phys' }] }
                ]
            }
        };

        const current: any = {
            '167': {
                group: '167',
                days: [
                    { day: '01.02.2026', lessons: [{ title: 'Math2' }] },
                    { day: '03.02.2026', lessons: [{ title: 'PE' }] }
                ]
            }
        };

        const out = diffGroups(current, previous, 10);
        expect(out).toContain('167: updated 01.02.2026');
        expect(out).toContain('167: added 03.02.2026');
        expect(out).toContain('167: removed 02.02.2026');
    });

    it('respects the max lines limit for teacher diffs', () => {
        const previous: any = {
            'Ivanov I.I.': {
                teacher: 'Ivanov I.I.',
                days: [{ day: '01.02.2026', lessons: [] }]
            }
        };

        const current: any = {
            'Ivanov I.I.': {
                teacher: 'Ivanov I.I.',
                days: [
                    { day: '01.02.2026', lessons: [{ title: 'A' }] },
                    { day: '02.02.2026', lessons: [] }
                ]
            }
        };

        const out = diffTeachers(current, previous, 1);
        expect(out).toHaveLength(1);
    });
});
