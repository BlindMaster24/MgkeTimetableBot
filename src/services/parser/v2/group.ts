import { config } from "../../../../config";
import { WeekIndex, getShortSubjectName, mergeDays } from "../../../utils";
import { AbstractParser } from "../abstract";
import { GroupDay, GroupLesson, GroupLessonExplain, Groups } from "../types/group";
import { buildTableGrid, findHeaderRowIndex, getDayRangesFromGrid } from "./grid";
import { DayRange, GridCell } from "./types";
import { cleanText, extractLines, isTeacherLine, parseLessonNumber } from "./text";

type ParsedTable = {
    group: string,
    groupNumber: string,
    days: GroupDay[]
}

export default class StudentParserV2 extends AbstractParser {
    protected groups: Groups = {};

    public run(): Groups {
        const tables = this.findTables();

        for (const table of tables) {
            const label = this.findHeading(table, 'группа');
            if (!label) {
                continue;
            }

            const parsed = this.parseTable(table, label);
            if (!parsed) {
                continue;
            }

            const existing = this.groups[parsed.groupNumber];
            if (existing) {
                if (config.parser.v2?.allowTwoTables === false) {
                    continue;
                }
                const merged = mergeDays(parsed.days, existing.days);
                existing.days = merged.mergedDays;
                existing.group = parsed.group;
            } else {
                this.groups[parsed.groupNumber] = {
                    group: parsed.group,
                    days: parsed.days
                };
            }
        }

        this.clearSundays(this.groups);

        for (const group in this.groups) {
            for (const day of this.groups[group].days) {
                this.postProcessDay(day);
            }
        }

        return this.groups;
    }

    protected parseTable(table: HTMLTableElement, label: string): ParsedTable | null {
        const group = this.parseGroupLabel(label);
        const groupNumber = group ? this.parseGroupNumber(group) : undefined;
        if (!group || !groupNumber) {
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

        const days: GroupDay[] = ranges.map((range) => ({
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
            group,
            groupNumber,
            days
        };
    }

    protected parseGroupLabel(label: string): string | null {
        const text = cleanText(label);
        if (!text) {
            return null;
        }

        if (!text.toLowerCase().startsWith('группа')) {
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

    protected assignLesson(day: GroupDay, lessonNumber: number, lesson: GroupLesson) {
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

    protected parseLesson(lessonCell?: HTMLTableCellElement | null, cabinetCell?: HTMLTableCellElement | null): GroupLesson {
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

        if (entries.length === 1 && entries[0].subgroup === undefined) {
            return entries[0];
        }

        return entries;
    }

    protected parseEntries(lines: string[]): GroupLessonExplain[] {
        const entries: GroupLessonExplain[] = [];
        let current: Partial<GroupLessonExplain> = {};

        const pushCurrent = () => {
            if (!current.lesson) {
                current = {};
                return;
            }

            entries.push({
                subgroup: current.subgroup,
                lesson: current.lesson,
                type: current.type ?? null,
                teacher: current.teacher ?? null,
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

            if (!current.lesson) {
                const parsed = this.parseLessonLine(raw);
                if (parsed) {
                    current.lesson = parsed.lesson;
                    current.subgroup = parsed.subgroup;
                }
                continue;
            }

            if (!current.teacher) {
                current.teacher = raw;
                continue;
            }

            if (isTeacherLine(raw)) {
                current.teacher = `${current.teacher}, ${raw}`;
                continue;
            }

            pushCurrent();
            const parsed = this.parseLessonLine(raw);
            if (parsed) {
                current.lesson = parsed.lesson;
                current.subgroup = parsed.subgroup;
            }
        }

        pushCurrent();

        return entries;
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

    protected parseCombinedLine(line: string): GroupLessonExplain | null {
        const match = line.match(/^(?:(\d+)\s*[.)]\s*)?(.+?)\s*\(([^)]+)\)\s*(.+)?$/);
        if (!match) {
            return null;
        }

        const subgroup = match[1] ? Number(match[1]) : undefined;
        const lesson = getShortSubjectName(match[2].trim());
        const type = match[3].trim();
        const teacher = cleanText(match[4]) ?? null;

        return {
            subgroup,
            lesson,
            type: type || null,
            teacher,
            cabinet: null,
            comment: null
        };
    }

    protected parseType(line: string): string | null {
        const match = line.match(/^\((.+)\)$/);
        return match ? match[1].trim() : null;
    }

    protected parseLessonLine(line: string): { lesson: string, subgroup?: number } | null {
        const match = line.match(/^(?:(\d+)\s*[.)]\s*)?(.+)$/);
        if (!match) {
            return null;
        }

        const subgroup = match[1] ? Number(match[1]) : undefined;
        const lesson = getShortSubjectName(match[2].trim());

        return {
            lesson,
            subgroup
        };
    }

    protected applyCabinets(entries: GroupLessonExplain[], cabinets: (string | null)[]) {
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

        const hasSubgroup = entries.some((entry) => entry.subgroup !== undefined);
        if (hasSubgroup) {
            for (let i = 0; i < entries.length; i++) {
                const entry = entries[i];
                if (entry.subgroup !== undefined && normalized[entry.subgroup - 1] !== undefined) {
                    entry.cabinet = normalized[entry.subgroup - 1] ?? null;
                } else {
                    entry.cabinet = normalized[Math.min(i, normalized.length - 1)] ?? null;
                }
            }
            return;
        }

        for (let i = 0; i < entries.length; i++) {
            entries[i].cabinet = normalized[i] ?? normalized[normalized.length - 1] ?? null;
        }
    }

    private postProcessDay(day: GroupDay) {
        for (let i = 0; i <= day.lessons.length; i++) {
            let lesson: GroupLesson = day.lessons[i];
            if (!lesson) continue;

            if (!Array.isArray(lesson)) {
                lesson = [lesson];
            }

            if (lesson.every((_) => _.type === 'ф-в' && _.comment == null)) {
                let simmilarIndex: number | null = null;

                let firstNotNull = false;
                for (let j: number = day.lessons.length; j > i; j--) {
                    let fLesson: GroupLesson = day.lessons[j];

                    if (firstNotNull && !fLesson) break;
                    if (!fLesson) continue;
                    firstNotNull = true;

                    if (!Array.isArray(fLesson)) {
                        fLesson = [fLesson];
                    }

                    if (lesson.length !== fLesson.length) continue;

                    for (let k = 0; k < lesson.length; k++) {
                        if (
                            lesson[k].type === fLesson[k].type &&
                            lesson[k].lesson === fLesson[k].lesson &&
                            lesson[k].teacher === fLesson[k].teacher
                        ) {
                            simmilarIndex = j;
                            break;
                        }
                    }

                    if (simmilarIndex !== null) {
                        break;
                    }
                }

                if (simmilarIndex !== null) {
                    lesson.forEach((_) => _.comment = '2 часа');
                    day.lessons[simmilarIndex] = null;
                }
            }
        }

        this.clearEndingNull(day.lessons);
    }
}
