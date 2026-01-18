import { config } from "../../../../config";
import { WeekIndex, getShortSubjectName, mergeDays } from "../../../utils";
import { AbstractParser } from "../abstract";
import { TeacherDay, TeacherLesson, TeacherLessonExplain, Teachers } from "../types/teacher";
import { buildTableGrid, findHeaderRowIndex, getDayRangesFromGrid } from "./grid";
import { DayRange, GridCell } from "./types";
import { cleanText, extractLines, parseLessonNumber } from "./text";

type ParsedTable = {
    teacher: string,
    days: TeacherDay[]
}

export default class TeacherParserV2 extends AbstractParser {
    protected teachers: Teachers = {};

    public run(): Teachers {
        const tables = this.findTables();

        for (const table of tables) {
            const label = this.findHeading(table, 'преподаватель');
            if (!label) {
                continue;
            }

            const parsed = this.parseTable(table, label);
            if (!parsed) {
                continue;
            }

            const existing = this.teachers[parsed.teacher];
            if (existing) {
                if (config.parser.v2?.allowTwoTables === false) {
                    continue;
                }
                const merged = mergeDays(parsed.days, existing.days);
                existing.days = merged.mergedDays;
                existing.teacher = parsed.teacher;
            } else {
                this.teachers[parsed.teacher] = {
                    teacher: parsed.teacher,
                    days: parsed.days
                };
            }
        }

        this.clearSundays(this.teachers);

        for (const teacher in this.teachers) {
            for (const day of this.teachers[teacher].days) {
                this.postProcessDay(day);
            }
        }

        return this.teachers;
    }

    protected parseTable(table: HTMLTableElement, label: string): ParsedTable | null {
        const teacher = this.parseTeacherLabel(label);
        if (!teacher) {
            return null;
        }

        const { grid } = buildTableGrid(table);
        const minDays = config.parser.v2?.minDaysInTable ?? 5;
        const headerRowIndex = findHeaderRowIndex(grid, config.parser.v2?.headerScanRows ?? 5, minDays);
        if (headerRowIndex === null) {
            return null;
        }
        const ranges = this.getDayRanges(grid, headerRowIndex);
        if (!ranges.length) {
            return null;
        }

        const days: TeacherDay[] = ranges.map((range) => ({
            day: range.day,
            lessons: []
        }));

        for (let rowIndex = headerRowIndex + 1; rowIndex < grid.length; rowIndex++) {
            const row = grid[rowIndex];
            if (!row) {
                continue;
            }

            const numberCell = row[0];
            if (!numberCell || numberCell.row !== rowIndex) {
                continue;
            }

            const lessonNumber = parseLessonNumber(numberCell.cell.textContent);
            if (!lessonNumber) {
                continue;
            }

            for (let i = 0; i < ranges.length; i++) {
                const range = ranges[i];
                const lessonCell = this.getCell(row, rowIndex, range.start);
                const cabinetCell = range.span > 1 ? this.getCell(row, rowIndex, range.start + 1) : undefined;

                const lesson = this.parseLesson(lessonCell, cabinetCell);
                this.assignLesson(days[i], lessonNumber, lesson);
            }
        }

        return {
            teacher,
            days
        };
    }

    protected parseTeacherLabel(label: string): string | null {
        const text = cleanText(label);
        if (!text) {
            return null;
        }

        if (!text.toLowerCase().startsWith('преподаватель')) {
            return null;
        }

        const parts = text.split('-');
        if (parts.length < 2) {
            return null;
        }

        return parts.slice(1).join('-').trim();
    }

    protected findTables(): HTMLTableElement[] {
        const tables = Array.from(this.content.querySelectorAll('table') as NodeListOf<HTMLTableElement>);
        const candidates: { table: HTMLTableElement, weekIndex: number }[] = [];
        const minDays = config.parser.v2?.minDaysInTable ?? 5;
        const headerScanRows = config.parser.v2?.headerScanRows ?? 5;

        for (const table of tables) {
            const { grid } = buildTableGrid(table);
            const headerRowIndex = findHeaderRowIndex(grid, headerScanRows, minDays);
            if (headerRowIndex === null) {
                continue;
            }

            const ranges = this.getDayRanges(grid, headerRowIndex);
            if (!ranges.length) {
                continue;
            }

            const weekIndexes = new Set<number>();
            for (const range of ranges) {
                try {
                    weekIndexes.add(WeekIndex.fromStringDate(range.day).valueOf());
                } catch {
                    continue;
                }
            }

            if (weekIndexes.size === 0) {
                continue;
            }

            const first = weekIndexes.values().next().value;
            if (typeof first !== 'number') {
                continue;
            }

            candidates.push({
                table,
                weekIndex: first
            });
        }

        if (!candidates.length) {
            return [];
        }

        const currentWeek = WeekIndex.now().valueOf();
        const holdCurrentOnSunday = config.parser.v2?.sundayHoldCurrent ?? true;
        const isSunday = new Date().getDay() === 0;
        const weekPolicy = config.parser.v2?.weekPolicy ?? 'preferCurrent';
        const weekIndexes = Array.from(new Set(candidates.map((item) => item.weekIndex)));
        let targetWeek = weekIndexes[0];

        if (weekIndexes.includes(currentWeek)) {
            targetWeek = currentWeek;
        } else if (weekPolicy === 'closest') {
            targetWeek = weekIndexes.sort((a, b) => Math.abs(a - currentWeek) - Math.abs(b - currentWeek))[0];
        } else if (weekPolicy === 'preferCurrent') {
            const future = weekIndexes.filter((value) => value > currentWeek).sort((a, b) => a - b);
            const past = weekIndexes.filter((value) => value < currentWeek).sort((a, b) => b - a);
            if (holdCurrentOnSunday && isSunday && past.length) {
                targetWeek = past[0];
            } else {
                targetWeek = future.length ? future[0] : weekIndexes.sort((a, b) => Math.abs(a - currentWeek) - Math.abs(b - currentWeek))[0];
            }
        }

        return candidates.filter((item) => item.weekIndex === targetWeek).map((item) => item.table);
    }

    protected findHeading(table: HTMLTableElement, keyword: string): string | null {
        let element = table.previousElementSibling;
        let depth = 0;

        while (element && depth < 12) {
            const tag = element.tagName.toLowerCase();
            if (['h1', 'h2', 'h3', 'h4'].includes(tag)) {
                const text = cleanText(element.textContent);
                if (text && text.toLowerCase().includes(keyword)) {
                    return text;
                }
            }

            if (tag === 'table') {
                break;
            }

            element = element.previousElementSibling;
            depth += 1;
        }

        return null;
    }

    protected getDayRanges(grid: GridCell[][], headerRowIndex: number): DayRange[] {
        const ranges = getDayRangesFromGrid(grid, headerRowIndex);
        const minDays = config.parser.v2?.minDaysInTable ?? 5;
        return ranges.length >= minDays ? ranges : [];
    }

    protected assignLesson(day: TeacherDay, lessonNumber: number, lesson: TeacherLesson) {
        const index = lessonNumber - 1;

        while (day.lessons.length < index) {
            day.lessons.push(null);
        }

        if (day.lessons.length === index) {
            day.lessons.push(lesson);
        } else {
            day.lessons[index] = lesson;
        }
    }

    protected parseLesson(lessonCell?: HTMLTableCellElement | null, cabinetCell?: HTMLTableCellElement | null): TeacherLesson {
        const lines = extractLines(lessonCell);
        if (!lines.length) {
            return null;
        }

        const entries = this.parseEntries(lines);
        if (!entries.length && config.parser.v2?.strict) {
            throw new Error('unable to parse lesson entries');
        }
        if (!entries.length) {
            return null;
        }

        const cabinets = extractLines(cabinetCell).map((value) => {
            return this.removeDashes(value) ?? null;
        });

        this.applyCabinets(entries, cabinets);

        return entries[0] ?? null;
    }

    protected parseEntries(lines: string[]): TeacherLessonExplain[] {
        const entries: TeacherLessonExplain[] = [];
        let current: Partial<TeacherLessonExplain> = {};

        const pushCurrent = () => {
            if (!current.lesson || !current.group) {
                current = {};
                return;
            }

            entries.push({
                lesson: current.lesson,
                type: current.type ?? null,
                subgroup: current.subgroup,
                group: current.group,
                cabinet: null,
                comment: null
            });

            current = {};
        };

        for (const raw of lines) {
            const combined = this.parseCombinedLine(raw);
            if (combined) {
                pushCurrent();
                entries.push(combined);
                continue;
            }

            const type = this.parseType(raw);
            if (type && current.lesson) {
                current.type = type;
                continue;
            }

            if (!current.lesson || !current.group) {
                const parsed = this.parseGroupLessonLine(raw);
                if (parsed) {
                    current.lesson = parsed.lesson;
                    current.group = parsed.group;
                    current.subgroup = parsed.subgroup;
                }
                continue;
            }

            pushCurrent();
            const parsed = this.parseGroupLessonLine(raw);
            if (parsed) {
                current.lesson = parsed.lesson;
                current.group = parsed.group;
                current.subgroup = parsed.subgroup;
            }
        }

        pushCurrent();

        return entries;
    }

    protected parseCombinedLine(line: string): TeacherLessonExplain | null {
        const match = line.match(/^(?:(\d+)\s*[.)]\s*)?(.+?)\s*-\s*(.+?)(?:\s*\(([^)]+)\))?\s*$/);
        if (!match) {
            return null;
        }

        const subgroupFromLine = match[1] ? Number(match[1]) : undefined;
        const groupPart = match[2].trim();
        const groupInfo = this.parseGroupPart(groupPart);
        const subgroup = subgroupFromLine ?? groupInfo.subgroup;
        const group = groupInfo.group;
        const lesson = getShortSubjectName(match[3].trim());
        const type = match[4]?.trim() ?? null;

        return {
            lesson,
            type,
            subgroup,
            group,
            cabinet: null,
            comment: null
        };
    }

    protected parseType(line: string): string | null {
        const match = line.match(/^\((.+)\)$/);
        return match ? match[1].trim() : null;
    }

    protected parseGroupLessonLine(line: string): { lesson: string, group: string, subgroup?: number } | null {
        const normalized = cleanText(line);
        if (!normalized) {
            return null;
        }

        const parts = normalized.split('-', 2);
        if (parts.length < 2) {
            return null;
        }

        const groupInfo = this.parseGroupPart(parts[0].trim());
        const lesson = getShortSubjectName(parts[1].trim());

        return {
            lesson,
            group: groupInfo.group,
            subgroup: groupInfo.subgroup
        };
    }

    protected parseGroupPart(value: string): { group: string, subgroup?: number } {
        const cleaned = value.replace(/\s+/g, '');
        const match = cleaned.match(/^(?:(\d+)\.)?(.+)$/);

        if (!match) {
            return { group: cleaned };
        }

        const subgroup = match[1] ? Number(match[1]) : undefined;
        const group = match[2].trim();

        return { group, subgroup };
    }

    protected applyCabinets(entries: TeacherLessonExplain[], cabinets: (string | null)[]) {
        if (entries.length === 0) {
            return;
        }

        const normalized = cabinets.length ? cabinets : [null];
        const hasAny = normalized.some((value) => value);
        if (!hasAny) {
            return;
        }

        if (normalized.length === 1) {
            for (const entry of entries) {
                entry.cabinet = normalized[0] ?? null;
            }
            return;
        }

        for (let i = 0; i < entries.length; i++) {
            entries[i].cabinet = normalized[i] ?? normalized[normalized.length - 1] ?? null;
        }
    }

    protected getCell(row: GridCell[], rowIndex: number, colIndex: number): HTMLTableCellElement | null {
        const cell = row[colIndex];
        if (!cell) {
            return null;
        }

        if (cell.row !== rowIndex) {
            return null;
        }

        return cell.cell;
    }

    private postProcessDay(day: TeacherDay) {
        for (let i: number = 0; i <= day.lessons.length; i++) {
            const lesson: TeacherLesson = day.lessons[i];
            if (!lesson) continue;

            if (lesson.type === 'ф-в' && lesson.comment == null) {
                let simmilarIndex: number | null = null;

                for (let j: number = day.lessons.length; j > i; j--) {
                    const fLesson: TeacherLesson = day.lessons[j];
                    if (!fLesson) continue;

                    if (
                        lesson.type === fLesson.type &&
                        lesson.lesson === fLesson.lesson &&
                        lesson.group === fLesson.group &&
                        lesson.subgroup === fLesson.subgroup
                    ) {
                        simmilarIndex = j;
                        break;
                    }
                }

                if (simmilarIndex !== null) {
                    lesson.comment = '2 часа';
                    day.lessons[simmilarIndex] = null;
                }
            }
        }

        this.clearEndingNull(day.lessons);
    }
}
