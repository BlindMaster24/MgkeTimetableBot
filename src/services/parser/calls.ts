import { DayCall } from "../../../config.scheme";

export type CallsSchedule = {
    weekdays: DayCall[];
    saturday: DayCall[];
};

export type ParsedCallsSchedule = {
    schedule: CallsSchedule;
    updatedAt?: number;
    updatedAtRaw?: string;
};

const timeRegex = /\b\d{1,2}[:.]\d{2}\b/g;

const normalizeTime = (value: string) => {
    const [h, m] = value.replace('.', ':').split(':');
    const hh = h.padStart(2, '0');
    return `${hh}:${m}`;
};

const parseRowTimes = (text: string): DayCall | null => {
    const normalized = text.replace(/(\d{1,2}[.:]\d{2})(\d{1,2}[.:]\d{2})/g, '$1 $2');
    const times = Array.from(normalized.matchAll(timeRegex)).map((match) => normalizeTime(match[0]));
    if (times.length < 4) {
        return null;
    }
    return [[times[0], times[1]], [times[2], times[3]]];
};

const extractTable = (table: Element): DayCall[] => {
    const calls: DayCall[] = [];
    const rows = Array.from(table.querySelectorAll('tr'));
    for (const row of rows) {
        const text = row.textContent?.replace(/\s+/g, ' ').trim() ?? '';
        if (!text) continue;
        const call = parseRowTimes(text);
        if (call) {
            calls.push(call);
        }
    }
    return calls;
};

const headingForTable = (table: Element): string => {
    let el: Element | null = table.previousElementSibling;
    let guard = 0;
    while (el && guard < 8) {
        const text = el.textContent?.toLowerCase().trim() ?? '';
        if (text) return text;
        el = el.previousElementSibling;
        guard += 1;
    }
    return '';
};

const parseUpdatedAt = (root: Element): { updatedAt?: number; updatedAtRaw?: string } => {
    const attr = root.querySelector('[date-updated]')?.getAttribute('date-updated')
        || root.getAttribute('date-updated')
        || null;
    const raw = attr ?? '';
    if (raw) {
        const match = /(\d{2}\.\d{2}\.\d{4})(?:\s+(\d{2}:\d{2}))?/.exec(raw);
        if (match) {
            const [day, month, year] = match[1].split('.');
            const time = match[2] ?? '00:00';
            const [hh, mm] = time.split(':');
            const date = new Date(Number(year), Number(month) - 1, Number(day), Number(hh), Number(mm));
            return { updatedAt: date.getTime(), updatedAtRaw: raw };
        }
    }
    return { updatedAt: undefined, updatedAtRaw: raw || undefined };
};

export const parseCallsSchedule = (doc: Document): ParsedCallsSchedule | null => {
    const candidates = [
        ...Array.from(doc.querySelectorAll('.entry .content')),
        ...Array.from(doc.querySelectorAll('#main-p .content')),
        ...Array.from(doc.querySelectorAll('.common-page-left-block .content')),
        ...Array.from(doc.querySelectorAll('.common-page-left-block'))
    ];

    let content: Element | null = null;
    for (let i = candidates.length - 1; i >= 0; i--) {
        const candidate = candidates[i];
        if (candidate.querySelector('table')) {
            content = candidate;
            break;
        }
    }

    const tables = content
        ? Array.from(content.querySelectorAll('table'))
        : Array.from(doc.querySelectorAll('main table, table'));
    if (!tables.length) return null;

    const weekdays: DayCall[] = [];
    const saturday: DayCall[] = [];

    for (const table of tables) {
        const heading = headingForTable(table).toLowerCase();
        const calls = extractTable(table);
        if (!calls.length) continue;

        if (heading.includes('\u0441\u0443\u0431\u0431\u043e\u0442')) {
            saturday.push(...calls);
        } else if (heading.includes('\u0432\u044b\u0445\u043e\u0434')) {
            saturday.push(...calls);
        } else {
            weekdays.push(...calls);
        }
    }

    const { updatedAt, updatedAtRaw } = parseUpdatedAt(content ?? doc.body);
    return {
        schedule: {
            weekdays,
            saturday: saturday.length ? saturday : weekdays
        },
        updatedAt,
        updatedAtRaw
    };
};
