import { describe, expect, it } from 'vitest';
import { config as exampleConfig } from '../../config.example';
import { validateConfig } from '../../src/config/validate';

const base = () => structuredClone(exampleConfig);

const enrichedBase = () => {
    const cfg: any = base();
    cfg.encrypt_key = Buffer.from('x'.repeat(32));
    cfg.vk ??= {
        app: { id: 0, secret: '', url: '' },
        bot: { id: 0, access_token: '', noticer: false },
        admin_ids: []
    };
    cfg.telegram = { ...cfg.telegram, token: cfg.telegram?.token || 'dummy-token' };
    cfg.viber ??= { name: '', token: '', url: '', admin_ids: [], noticer: false };
    cfg.alice ??= {};
    cfg.google ??= {
        redirectDomain: '',
        url: '',
        oauth: { clientId: '', clientSecret: '' },
        service_account: { clientEmail: '', privateKey: '' },
        calendar_owners: [],
        rateLimitter: { maxRequestsPerInterval: 1, interval: 1 }
    };
    cfg.calendar ??= { ics: { enabled: false } };
    cfg.accept ??= { room: false, private: false, app: false };
    cfg.api ??= { url: 'http://localhost' };
    cfg.globalNoticer ??= false;
    cfg.globalAdblock ??= false;
    return cfg;
};

describe('validateConfig', () => {
    it('accepts the example config augmented with required keys', () => {
        const cfg = enrichedBase();
        expect(() => validateConfig(cfg)).not.toThrow();
    });

    it('rejects missing telegram.token', () => {
        const cfg = enrichedBase();
        delete cfg.telegram.token;
        expect(() => validateConfig(cfg)).toThrow(/telegram/);
    });

    it('rejects missing encrypt_key', () => {
        const cfg = enrichedBase();
        delete cfg.encrypt_key;
        expect(() => validateConfig(cfg)).toThrow(/encrypt_key/);
    });

    it('rejects non-Buffer encrypt_key', () => {
        const cfg = enrichedBase();
        cfg.encrypt_key = 'not a buffer';
        expect(() => validateConfig(cfg)).toThrow(/encrypt_key/);
    });

    it('rejects out-of-range http.port', () => {
        const cfg = enrichedBase();
        cfg.http.port = 999999;
        expect(() => validateConfig(cfg)).toThrow(/http/);
    });

    it('rejects unknown service name in services array', () => {
        const cfg = enrichedBase();
        cfg.services.push('not_a_real_service');
        expect(() => validateConfig(cfg)).toThrow(/services/);
    });

    it('rejects invalid logging.level', () => {
        const cfg = enrichedBase();
        cfg.logging.level = 'fatal';
        expect(() => validateConfig(cfg)).toThrow(/logging/);
    });

    it('rejects invalid parser.v2.weekPolicy', () => {
        const cfg = enrichedBase();
        cfg.parser.v2.weekPolicy = 'bogus';
        expect(() => validateConfig(cfg)).toThrow(/parser/);
    });
});
