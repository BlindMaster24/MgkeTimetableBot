import assert from 'assert';
import { diffGroups, diffTeachers } from '../src/services/parser/v2/diff';

function testGroupDiff() {
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
    assert.ok(out.includes('167: updated 01.02.2026'));
    assert.ok(out.includes('167: added 03.02.2026'));
    assert.ok(out.includes('167: removed 02.02.2026'));
}

function testTeacherDiffLimit() {
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
    assert.strictEqual(out.length, 1);
}

(() => {
    console.log('parserV2DiffTest: start');
    testGroupDiff();
    console.log('parserV2DiffTest: group-diff ok');
    testTeacherDiffLimit();
    console.log('parserV2DiffTest: teacher-limit ok');
    console.log('parserV2DiffTest: ok');
})();
