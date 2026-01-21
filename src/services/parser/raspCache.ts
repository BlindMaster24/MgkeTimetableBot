import { existsSync, readFileSync, unlinkSync } from 'fs';
import { mkdir, writeFile } from 'fs/promises';
import { Groups, Teachers, Team } from '../timetable';
import { CallsSchedule } from './calls';

export type RaspEntryCache<T = Groups | Teachers> = {
    timetable: T,
    update: number,
    changed: number,
    lastWeekIndex: number,
    hash: string,
}

export type TeamCache = {
    names: Team,
    update: number,
    changed: number,
    hash: string[]
}

export type CallsCache = {
    site: {
        schedule: CallsSchedule,
        updatedAt: number,
        updatedAtRaw?: string,
        hash: string
    },
    manual: {
        schedule: CallsSchedule,
        updatedAt: number,
        hash: string
    },
    active: {
        schedule: CallsSchedule,
        updatedAt: number,
        source: 'site' | 'manual' | 'config',
        hash: string
    },
    overrideSource?: 'site' | 'manual' | 'config',
    manualReason?: string,
    manualReasonUpdatedAt?: number,
    siteEmptyNotifiedAt?: number,
    update: number,
    changed: number
}

export type RaspCache = {
    groups: RaspEntryCache<Groups>,
    teachers: RaspEntryCache<Teachers>,
    team: TeamCache,
    calls: CallsCache,
    successUpdate: boolean
}

export const raspCache: RaspCache = {
    groups: {
        timetable: {},
        changed: 0,
        update: 0,
        lastWeekIndex: 0,
        hash: ''
    },
    teachers: {
        timetable: {},
        changed: 0,
        update: 0,
        lastWeekIndex: 0,
        hash: ''
    },
    team: {
        names: {},
        update: 0,
        changed: 0,
        hash: []
    },
    calls: {
        site: {
            schedule: { weekdays: [], saturday: [] },
            updatedAt: 0,
            hash: ''
        },
        manual: {
            schedule: { weekdays: [], saturday: [] },
            updatedAt: 0,
            hash: ''
        },
        active: {
            schedule: { weekdays: [], saturday: [] },
            updatedAt: 0,
            source: 'config',
            hash: ''
        },
        update: 0,
        changed: 0
    },
    successUpdate: true
}

export async function saveCache() {
    if (!existsSync('./cache/rasp/')) {
        await mkdir('./cache/rasp/', { recursive: true });
    }

    await writeFile('./cache/rasp/groups.json', JSON.stringify(raspCache.groups, null, 4));
    await writeFile('./cache/rasp/teachers.json', JSON.stringify(raspCache.teachers, null, 4));
    await writeFile('./cache/rasp/team.json', JSON.stringify(raspCache.team, null, 4))
    await writeFile('./cache/rasp/calls.json', JSON.stringify(raspCache.calls, null, 4))
}

export function loadCache() {
    if (!existsSync('./cache/rasp/')) return;

    if (existsSync('./cache/rasp/groups.json')) {
        try {
            Object.assign(raspCache.groups, JSON.parse(readFileSync('./cache/rasp/groups.json', 'utf8')));
        } catch (e) {
            unlinkSync('./cache/rasp/groups.json')
        }
    }

    if (existsSync('./cache/rasp/teachers.json')) {
        try {
            Object.assign(raspCache.teachers, JSON.parse(readFileSync('./cache/rasp/teachers.json', 'utf8')));
        } catch (e) {
            unlinkSync('./cache/rasp/teachers.json')
        }
    }

    if (existsSync('./cache/rasp/team.json')) {
        try {
            Object.assign(raspCache.team, JSON.parse(readFileSync('./cache/rasp/team.json', 'utf8')));
        } catch (e) {
            unlinkSync('./cache/rasp/team.json')
        }
    }

    if (existsSync('./cache/rasp/calls.json')) {
        try {
            Object.assign(raspCache.calls, JSON.parse(readFileSync('./cache/rasp/calls.json', 'utf8')));
        } catch (e) {
            unlinkSync('./cache/rasp/calls.json')
        }
    }
}
