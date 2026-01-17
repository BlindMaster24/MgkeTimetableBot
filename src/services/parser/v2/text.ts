import { StringDate } from "../../../utils";

const DAY_NAMES = [
    'понедельник',
    'вторник',
    'среда',
    'четверг',
    'пятница',
    'суббота',
    'воскресенье'
];

const DATE_RE = /(\d{1,2}\.\d{1,2}\.\d{2,4})/;

export function cleanText(text?: string | null): string | null {
    if (text == null) {
        return null;
    }

    const normalized = text.replace(/\s+/g, ' ').trim();
    return normalized.length ? normalized : null;
}

export function normalizeDate(value: string): string | null {
    const parts = value.split('.').map((part) => part.trim()).filter(Boolean);
    if (parts.length < 3) {
        return null;
    }

    let [day, month, year] = parts;
    if (year.length === 2) {
        year = `20${year}`;
    }

    const dayNum = Number(day);
    const monthNum = Number(month);
    const yearNum = Number(year);

    if (!dayNum || !monthNum || !yearNum) {
        return null;
    }

    const normalized = `${dayNum.toString().padStart(2, '0')}.${monthNum.toString().padStart(2, '0')}.${yearNum}`;
    try {
        return StringDate.fromStringDate(normalized).toStringDate();
    } catch {
        return null;
    }
}

export function parseDayLabel(value?: string | null): { day: string, weekday?: string } | null {
    const text = cleanText(value);
    if (!text) {
        return null;
    }

    const dateMatch = text.match(DATE_RE);
    if (!dateMatch) {
        return null;
    }

    const normalized = normalizeDate(dateMatch[1]);
    if (!normalized) {
        return null;
    }

    const lower = text.toLowerCase();
    const weekdayMatch = DAY_NAMES.find((name) => lower.includes(name));
    const weekday = weekdayMatch ? capitalize(weekdayMatch) : undefined;

    return {
        day: normalized,
        weekday
    };
}

function capitalize(text: string): string {
    return text.charAt(0).toUpperCase() + text.slice(1);
}

export function extractLines(cell?: HTMLTableCellElement | null): string[] {
    if (!cell) {
        return [];
    }

    const lines: string[] = [];
    let buffer = '';

    const flush = () => {
        const normalized = cleanText(buffer);
        if (normalized) {
            lines.push(normalized);
        }
        buffer = '';
    };

    const walk = (node: Node) => {
        if (node.nodeType === node.TEXT_NODE) {
            buffer += node.textContent ?? '';
            return;
        }

        if (node.nodeType !== node.ELEMENT_NODE) {
            return;
        }

        const element = node as Element;
        const tag = element.tagName.toLowerCase();

        if (tag === 'br') {
            flush();
            return;
        }

        const isBlock = tag === 'p' || tag === 'div' || tag === 'tr' || tag === 'li';

        if (isBlock) {
            flush();
        }

        for (const child of Array.from(element.childNodes)) {
            walk(child);
        }

        if (isBlock) {
            flush();
        }
    };

    walk(cell);
    flush();

    return lines;
}

export function parseLessonNumber(value?: string | null): number | null {
    const text = cleanText(value);
    if (!text) {
        return null;
    }

    const match = text.match(/(\d+)/);
    if (!match) {
        return null;
    }

    const num = Number(match[1]);
    return Number.isFinite(num) ? num : null;
}

export function isTeacherLine(value: string): boolean {
    const text = value.trim();
    if (!text) {
        return false;
    }

    return /^[A-ZА-ЯЁ][a-zа-яё-]+\s+[A-ZА-ЯЁ]\.\s*[A-ZА-ЯЁ]?\.?$/.test(text);
}
