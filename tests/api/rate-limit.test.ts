import express from 'express';
import request from 'supertest';
import { describe, expect, it } from 'vitest';
import { createApiRateLimiter } from '../../src/services/api';

const mkServer = (max: number) => {
    const srv = express();
    srv.use('/api/:method', createApiRateLimiter({ windowMs: 60_000, max }));
    srv.get('/api/:method', (_req, res) => res.json({ ok: true }));
    return srv;
};

describe('createApiRateLimiter', () => {
    it('returns 429 after the configured max is exceeded for anonymous ip', async () => {
        const srv = mkServer(3);
        const agent = request.agent(srv);

        for (let i = 0; i < 3; i++) {
            const res = await agent.get('/api/info');
            expect(res.status).toBe(200);
        }
        const overflow = await agent.get('/api/info');
        expect(overflow.status).toBe(429);
        expect(overflow.body.error).toBe('Превышен лимит запросов');
    });

    it('limits by ip regardless of authorization header (prevents token rotation bypass)', async () => {
        const srv = mkServer(3);
        const agent = request.agent(srv);

        for (let i = 0; i < 3; i++) {
            const res = await agent.get('/api/info').set('authorization', `Bearer rotating-${i}`);
            expect(res.status).toBe(200);
        }
        const overflow = await agent.get('/api/info').set('authorization', 'Bearer rotating-overflow');
        expect(overflow.status).toBe(429);
    });

    it('emits RateLimit standard headers on successful requests', async () => {
        const srv = mkServer(5);
        const res = await request(srv).get('/api/info');

        expect(res.status).toBe(200);
        expect(res.headers['ratelimit']).toBeDefined();
        expect(res.headers['x-ratelimit-limit']).toBeUndefined();
    });
});
