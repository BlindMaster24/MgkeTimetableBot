import { describe, expect, it } from 'vitest';
import type { App } from '../../src/app';
import { CompactScheduleFormatter } from '../../src/formatter/compact';
import { DefaultScheduleFormatter } from '../../src/formatter/default';
import { LitolaxScheduleFormatter } from '../../src/formatter/litolax';
import { VisualScheduleFormatter } from '../../src/formatter/visual';
import type { GroupLesson, TeacherLesson } from '../../src/services/parser/types';
import type { RaspCache } from '../../src/services/parser/raspCache';

const fakeRasp = (): RaspCache =>
    ({
        successUpdate: true,
        groups: { update: Date.now(), lastWeekIndex: 0, timetable: {} },
        teachers: { update: Date.now(), lastWeekIndex: 0, timetable: {} },
        team: { update: 0, names: { Ivanov: 'Иванов И.И.' } }
    }) as unknown as RaspCache;

const fakeApp = (): App =>
    ({
        getService: () => ({ isHasErrors: () => false })
    }) as unknown as App;

const groupLessons: GroupLesson[] = [
    null as unknown as GroupLesson,
    {
        lesson: 'Математика',
        type: 'лек',
        teacher: 'Петров',
        cabinet: '301',
        comment: null,
        subgroup: null
    } as unknown as GroupLesson,
    [
        {
            lesson: 'Английский',
            type: 'пр',
            teacher: 'Иванов',
            cabinet: '201',
            comment: null,
            subgroup: 1
        },
        {
            lesson: 'Английский',
            type: 'пр',
            teacher: 'Сидоров',
            cabinet: '202',
            comment: null,
            subgroup: 2
        }
    ] as unknown as GroupLesson
];

const teacherLessons: TeacherLesson[] = [
    {
        lesson: 'Математика',
        type: 'лек',
        group: 'ПС-11',
        cabinet: '301',
        comment: null,
        subgroup: null
    } as unknown as TeacherLesson,
    {
        lesson: 'Алгебра',
        type: 'пр',
        group: 'ПС-12',
        cabinet: '205',
        comment: 'тест',
        subgroup: 1
    } as unknown as TeacherLesson
];

const FORMATTERS = [
    ['default', DefaultScheduleFormatter],
    ['compact', CompactScheduleFormatter],
    ['visual', VisualScheduleFormatter],
    ['litolax', LitolaxScheduleFormatter]
] as const;

describe.each(FORMATTERS)('%s formatter', (name, FormatterClass) => {
    const make = (service: 'tg' | 'vk' = 'vk') => new FormatterClass(service, fakeApp(), fakeRasp(), undefined);

    it('returns a "no lessons" placeholder when lessons are empty', () => {
        const out = make().formatGroupLessons([]);
        expect(out).toMatch(/Пар нет|Нет пар/);
    });

    it('returns a "no lessons" placeholder when lessons are undefined', () => {
        const out = make().formatGroupLessons(undefined);
        expect(out).toMatch(/Пар нет|Нет пар/);
    });

    it('formats a single group lesson with main fields', () => {
        const out = make().formatGroupLessons([groupLessons[1]]);
        expect(out).toContain('Математика');
        expect(out).toMatch(/301|лек/);
    });

    it('formats a group lesson with two subgroups', () => {
        const out = make().formatGroupLessons([groupLessons[2]]);
        expect(out).toContain('Английский');
        void name;
    });

    it('formats a teacher lesson with group and cabinet', () => {
        const out = make().formatTeacherLessons(teacherLessons);
        expect(out).toContain('Математика');
        expect(out).toContain('ПС-11');
    });

    it('applies HTML bold wrapping only for telegram', () => {
        const tg = make('tg');
        const vk = make('vk');

        expect(tg.b('X')).toBe('<b>X</b>');
        expect(vk.b('X')).toBe('X');
    });

    it('exposes a static label describing the formatter in menus', () => {
        expect(FormatterClass.label).toMatch(/./);
        expect(typeof FormatterClass.label).toBe('string');
    });
});

describe('DefaultScheduleFormatter fine-grained output', () => {
    const make = () => new DefaultScheduleFormatter('vk', fakeApp(), fakeRasp(), undefined);

    it('uses tree branches for subgroups', () => {
        const out = make().formatGroupLessons([groupLessons[2]]);
        expect(out).toMatch(/├──|└──/);
    });

    it('uses numbered prefix for lesson headers (1., 2., …)', () => {
        const out = make().formatGroupLessons([groupLessons[1], groupLessons[2]]);
        expect(out).toMatch(/^1\.\s/m);
        expect(out).toMatch(/^2\.\s/m);
    });
});

describe('CompactScheduleFormatter fine-grained output', () => {
    const make = () => new CompactScheduleFormatter('vk', fakeApp(), fakeRasp(), undefined);

    it('omits lesson type and teacher from output', () => {
        const out = make().formatGroupLessons([groupLessons[1]]);
        expect(out).toContain('Математика');
        expect(out).not.toContain('лек');
        expect(out).not.toContain('Петров');
    });

    it('uses simple dash prefix for subgroups', () => {
        const out = make().formatGroupLessons([groupLessons[2]]);
        expect(out).toMatch(/^-\s/m);
    });
});

describe('LitolaxScheduleFormatter fine-grained output', () => {
    const make = () => new LitolaxScheduleFormatter('vk', fakeApp(), fakeRasp(), undefined);

    it('adds Каб: footer with cabinet numbers after each group lesson block', () => {
        const out = make().formatGroupLessons([groupLessons[2]]);
        expect(out).toContain('Каб:');
        expect(out).toContain('201');
        expect(out).toContain('202');
    });
});
