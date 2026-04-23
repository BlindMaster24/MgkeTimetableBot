import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync } from 'fs';
import path from 'path';
import { JSDOM } from 'jsdom';
import StudentParserV2 from '../../src/services/parser/v2/group';
import TeacherParserV2 from '../../src/services/parser/v2/teacher';

type FixtureMeta = {
    type: 'group' | 'teacher',
    expected: unknown
};

const fixturesDir = path.join(__dirname, '..', 'fixtures', 'parser-v2');
const htmlFiles = existsSync(fixturesDir)
    ? readdirSync(fixturesDir).filter((file) => file.endsWith('.html'))
    : [];

describe('parser v2 fixtures', () => {
    if (!existsSync(fixturesDir)) {
        it.skip('fixtures directory is missing', () => {});
        return;
    }

    if (htmlFiles.length === 0) {
        it.skip('no fixtures found', () => {});
        return;
    }

    for (const file of htmlFiles) {
        const name = path.basename(file, '.html');
        const htmlPath = path.join(fixturesDir, file);
        const metaPath = path.join(fixturesDir, `${name}.json`);

        it(`parses fixture ${name}`, () => {
            expect(existsSync(metaPath), `missing meta for ${file}`).toBe(true);

            const html = readFileSync(htmlPath, 'utf8');
            const meta = JSON.parse(readFileSync(metaPath, 'utf8')) as FixtureMeta;

            const dom = new JSDOM(html);
            const parser = meta.type === 'group'
                ? new StudentParserV2(dom.window)
                : new TeacherParserV2(dom.window);
            const actual = parser.run();

            expect(JSON.parse(JSON.stringify(actual))).toEqual(meta.expected);
        });
    }
});
