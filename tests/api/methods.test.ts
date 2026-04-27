import express, { Express, NextFunction, Request, Response } from 'express';
import request from 'supertest';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { App } from '../../src/app';
import GetGroupMethod from '../../src/services/api/methods/getGroup';
import GetGroupsMethod from '../../src/services/api/methods/getGroups';
import GetTeacherMethod from '../../src/services/api/methods/getTeacher';
import GetTeachersMethod from '../../src/services/api/methods/getTeachers';
import InfoMethod from '../../src/services/api/methods/info';
import ParserHealthMethod from '../../src/services/api/methods/parser-health';
import { raspCache } from '../../src/services/parser/raspCache';

type FakeTimetable = {
    getGroups: () => Promise<string[]>;
    getTeachers: () => Promise<string[]>;
};

const mkApp = (timetable: Partial<FakeTimetable> = {}): App =>
    ({
        getService: (name: string) => {
            if (name === 'timetable') {
                return {
                    getGroups: timetable.getGroups ?? (async () => []),
                    getTeachers: timetable.getTeachers ?? (async () => [])
                };
            }
            throw new Error(`Unknown service: ${name}`);
        }
    }) as unknown as App;

const mountHandler = (
    httpMethod: 'GET' | 'POST',
    path: string,
    method: { handler: (params: { app: App; request: Request; response: Response }) => unknown },
    app: App
): Express => {
    const srv = express();
    srv.use(express.json());
    const wrap = async (req: Request, res: Response, next: NextFunction) => {
        try {
            const result = await method.handler({ app, request: req, response: res });
            if (result === undefined) return;
            res.json({ response: result });
        } catch (err) {
            next(err);
        }
    };
    if (httpMethod === 'GET') srv.get(path, wrap);
    else srv.post(path, wrap);
    srv.use((err: unknown, _req: Request, res: Response, _next: NextFunction) => {
        if (err && typeof err === 'object' && 'name' in err && (err as { name: string }).name === 'ZodError') {
            return res.status(400).json({ error: (err as Error).message });
        }
        res.status(500).json({ error: 'server error' });
    });
    return srv;
};

const resetRaspCache = () => {
    raspCache.groups.timetable = {};
    raspCache.groups.update = 0;
    raspCache.groups.changed = 0;
    raspCache.groups.lastWeekIndex = 0;
    raspCache.groups.hash = '';
    raspCache.teachers.timetable = {};
    raspCache.teachers.update = 0;
    raspCache.teachers.changed = 0;
    raspCache.teachers.lastWeekIndex = 0;
    raspCache.teachers.hash = '';
    raspCache.team.names = {};
    raspCache.team.update = 0;
    raspCache.team.changed = 0;
    raspCache.team.hash = [];
    raspCache.successUpdate = true;
};

beforeEach(() => {
    resetRaspCache();
});

afterEach(() => {
    resetRaspCache();
});

describe('GET /getGroups', () => {
    it('returns a sorted list of groups from the timetable service', async () => {
        const app = mkApp({ getGroups: async () => ['ПС-11', 'А-21', 'ПС-12'] });
        const srv = mountHandler('GET', '/getGroups', new GetGroupsMethod(), app);

        const res = await request(srv).get('/getGroups');
        expect(res.status).toBe(200);
        expect(res.body.response).toEqual(['А-21', 'ПС-11', 'ПС-12']);
    });

    it('returns an empty array when no groups are known', async () => {
        const app = mkApp({ getGroups: async () => [] });
        const srv = mountHandler('GET', '/getGroups', new GetGroupsMethod(), app);

        const res = await request(srv).get('/getGroups');
        expect(res.status).toBe(200);
        expect(res.body.response).toEqual([]);
    });
});

describe('GET /getTeachers', () => {
    it('returns a sorted list of teachers from the timetable service', async () => {
        const app = mkApp({ getTeachers: async () => ['Петров П.П.', 'Иванов И.И.'] });
        const srv = mountHandler('GET', '/getTeachers', new GetTeachersMethod(), app);

        const res = await request(srv).get('/getTeachers');
        expect(res.status).toBe(200);
        expect(res.body.response).toEqual(['Иванов И.И.', 'Петров П.П.']);
    });
});

describe('POST /getGroup', () => {
    it('returns days enriched with a weekday for the requested group', async () => {
        raspCache.groups.update = 1234;
        raspCache.groups.changed = 5678;
        raspCache.groups.timetable = {
            'ПС-11': {
                group: 'ПС-11',
                days: [{ day: '02.09.2024', lessons: [] }]
            }
        } as unknown as typeof raspCache.groups.timetable;

        const srv = mountHandler('POST', '/getGroup', new GetGroupMethod(), mkApp());

        const res = await request(srv).post('/getGroup').send({ group: 'ПС-11' });
        expect(res.status).toBe(200);
        expect(res.body.response.days).toHaveLength(1);
        expect(res.body.response.days[0].day).toBe('02.09.2024');
        expect(res.body.response.days[0].weekday).toMatch(/./);
        expect(res.body.response.update).toBe(1234);
        expect(res.body.response.lastSuccess).toBe(true);
    });

    it('returns days=null for unknown groups', async () => {
        const srv = mountHandler('POST', '/getGroup', new GetGroupMethod(), mkApp());
        const res = await request(srv).post('/getGroup').send({ group: 'UNKNOWN' });
        expect(res.status).toBe(200);
        expect(res.body.response.days).toBeNull();
    });

    it('returns 400 when the group parameter is missing', async () => {
        const srv = mountHandler('POST', '/getGroup', new GetGroupMethod(), mkApp());
        const res = await request(srv).post('/getGroup').send({});
        expect(res.status).toBe(400);
    });

    it('accepts a numeric group id and coerces it to string', async () => {
        raspCache.groups.timetable = {
            '21': { group: '21', days: [{ day: '02.09.2024', lessons: [] }] }
        } as unknown as typeof raspCache.groups.timetable;

        const srv = mountHandler('POST', '/getGroup', new GetGroupMethod(), mkApp());
        const res = await request(srv).post('/getGroup').send({ group: 21 });
        expect(res.status).toBe(200);
        expect(res.body.response.days).toHaveLength(1);
    });
});

describe('POST /getTeacher', () => {
    it('returns days enriched with a weekday for the requested teacher', async () => {
        raspCache.teachers.update = 100;
        raspCache.teachers.timetable = {
            Ivanov: {
                teacher: 'Ivanov',
                days: [{ day: '02.09.2024', lessons: [] }]
            }
        } as unknown as typeof raspCache.teachers.timetable;

        const srv = mountHandler('POST', '/getTeacher', new GetTeacherMethod(), mkApp());
        const res = await request(srv).post('/getTeacher').send({ teacher: 'Ivanov' });

        expect(res.status).toBe(200);
        expect(res.body.response.days).toHaveLength(1);
        expect(res.body.response.days[0].weekday).toMatch(/./);
        expect(res.body.response.update).toBe(100);
    });

    it('returns 400 when teacher is missing', async () => {
        const srv = mountHandler('POST', '/getTeacher', new GetTeacherMethod(), mkApp());
        const res = await request(srv).post('/getTeacher').send({});
        expect(res.status).toBe(400);
    });
});

describe('GET /info', () => {
    it('returns cache metadata for groups, teachers, and team', async () => {
        raspCache.groups.update = 1;
        raspCache.groups.changed = 2;
        raspCache.groups.lastWeekIndex = 3;
        raspCache.groups.hash = 'gh';
        raspCache.teachers.update = 4;
        raspCache.teachers.changed = 5;
        raspCache.teachers.lastWeekIndex = 6;
        raspCache.teachers.hash = 'th';
        raspCache.team.update = 7;
        raspCache.team.changed = 8;
        raspCache.team.hash = ['a', 'b'];
        raspCache.successUpdate = true;

        const srv = mountHandler('GET', '/info', new InfoMethod(), mkApp());
        const res = await request(srv).get('/info');
        expect(res.status).toBe(200);
        expect(res.body.response).toMatchObject({
            groups: { update: 1, changed: 2, lastWeekIndex: 3, hash: 'gh' },
            teachers: { update: 4, changed: 5, lastWeekIndex: 6, hash: 'th' },
            team: { update: 7, changed: 8, hash: ['a', 'b'] },
            lastSuccess: true
        });
    });
});

describe('GET /parser-health', () => {
    it('reports ok=true when successUpdate is true and returns cache metadata', async () => {
        raspCache.successUpdate = true;
        raspCache.groups.update = 1234;
        raspCache.teachers.update = 1234;

        const srv = mountHandler('GET', '/parser-health', new ParserHealthMethod(), mkApp());
        const res = await request(srv).get('/parser-health');
        expect(res.status).toBe(200);
        expect(res.body.response.ok).toBe(true);
        expect(res.body.response.lastSuccessUpdate).toBe(1234);
        expect(res.body.response.metrics).toHaveProperty('student');
        expect(res.body.response.metrics).toHaveProperty('teacher');
    });

    it('reports ok=false and lastSuccessUpdate=0 when successUpdate is false', async () => {
        raspCache.successUpdate = false;
        const srv = mountHandler('GET', '/parser-health', new ParserHealthMethod(), mkApp());
        const res = await request(srv).get('/parser-health');
        expect(res.status).toBe(200);
        expect(res.body.response.ok).toBe(false);
        expect(res.body.response.lastSuccessUpdate).toBe(0);
    });
});
