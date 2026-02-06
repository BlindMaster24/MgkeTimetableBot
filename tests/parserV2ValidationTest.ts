import assert from 'assert';
import { validateGroups, validateTeachers } from '../src/services/parser/v2/validate';

function testEmpty() {
    const groups = validateGroups({}, 6, 0);
    assert.strictEqual(groups.ok, false);
    assert.ok(groups.errors.includes('empty groups'));

    const teachers = validateTeachers({}, 6, 0);
    assert.strictEqual(teachers.ok, false);
    assert.ok(teachers.errors.includes('empty teachers'));
}

function testInvalidGroupDays() {
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
    assert.strictEqual(out.ok, false);
    const errors = out.errors.join(' ');
    assert.ok(errors.includes('duplicate day'));
    assert.ok(errors.includes('too many lessons'));
}

function testNoLessonsSample() {
    const teachers: any = {
        'Ivanov I.I.': {
            teacher: 'Ivanov I.I.',
            days: [{ day: '01.02.2026', lessons: [] }]
        }
    };

    const out = validateTeachers(teachers, 6, 0);
    assert.strictEqual(out.ok, false);
    assert.ok(out.errors.includes('no lessons in sample'));
}

(() => {
    console.log('parserV2ValidationTest: start');
    testEmpty();
    console.log('parserV2ValidationTest: empty-case ok');
    testInvalidGroupDays();
    console.log('parserV2ValidationTest: invalid-days-case ok');
    testNoLessonsSample();
    console.log('parserV2ValidationTest: no-lessons-case ok');
    console.log('parserV2ValidationTest: ok');
})();
