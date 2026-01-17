import { existsSync, readFileSync, readdirSync } from "fs";
import path from "path";
import { JSDOM } from "jsdom";
import StudentParserV2 from "../src/services/parser/v2/group";
import TeacherParserV2 from "../src/services/parser/v2/teacher";

type FixtureMeta = {
    type: "group" | "teacher",
    expected: unknown
}

const fixturesDir = path.join(__dirname, "fixtures", "parser-v2");
if (!existsSync(fixturesDir)) {
    console.log("fixtures dir missing");
    process.exit(1);
}

const htmlFiles = readdirSync(fixturesDir).filter((file) => file.endsWith(".html"));
if (htmlFiles.length === 0) {
    console.log("no fixtures");
    process.exit(0);
}

let failed = 0;

for (const file of htmlFiles) {
    const name = path.basename(file, ".html");
    const htmlPath = path.join(fixturesDir, file);
    const metaPath = path.join(fixturesDir, `${name}.json`);

    if (!existsSync(metaPath)) {
        console.log(`missing meta for ${file}`);
        failed += 1;
        continue;
    }

    const html = readFileSync(htmlPath, "utf8");
    const meta = JSON.parse(readFileSync(metaPath, "utf8")) as FixtureMeta;

    const dom = new JSDOM(html);
    const parser = meta.type === "group" ? new StudentParserV2(dom.window) : new TeacherParserV2(dom.window);
    const actual = parser.run();

    const actualJson = JSON.stringify(actual);
    const expectedJson = JSON.stringify(meta.expected);

    if (actualJson !== expectedJson) {
        console.log(`fixture ${name} failed`);
        console.log(`expected: ${expectedJson}`);
        console.log(`actual: ${actualJson}`);
        failed += 1;
    } else {
        console.log(`fixture ${name} ok`);
    }
}

if (failed > 0) {
    console.log(`failed: ${failed}`);
    process.exit(1);
}

console.log("all fixtures ok");
