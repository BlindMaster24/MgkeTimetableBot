import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../config', () => ({
    config: {
        http: { port: 0 },
        dev: false,
        api: { url: '/api', rateLimit: { enabled: false } }
    }
}));

const closeMock = vi.fn().mockResolvedValue(undefined);
vi.mock('../../src/db', () => ({
    sequelize: { close: closeMock, sync: vi.fn().mockResolvedValue(undefined) }
}));
vi.mock('../../src/db/migrator', () => ({
    runMigrations: vi.fn().mockResolvedValue([])
}));

vi.mock('../../src/http', () => ({ HttpService: class {} }));
vi.mock('../../src/services/alice', () => ({ AliceApp: class {} }));
vi.mock('../../src/services/api', () => ({ Api: class {} }));
vi.mock('../../src/services/bots', () => ({ BotService: class {} }));
vi.mock('../../src/services/bots/tg', () => ({ TgBot: class {} }));
vi.mock('../../src/services/bots/viber', () => ({ ViberBot: class {} }));
vi.mock('../../src/services/bots/vk', () => ({ VkBot: class {} }));
vi.mock('../../src/services/google', () => ({ GoogleService: class {} }));
vi.mock('../../src/services/image', () => ({ ImageService: class {} }));
vi.mock('../../src/services/parser', () => ({ ParserService: class {} }));
vi.mock('../../src/services/timetable', () => ({ Timetable: class {} }));
vi.mock('../../src/services/vk_app', () => ({ VKApp: class {} }));

const loadApp = async () => {
    const { App } = await import('../../src/app');
    return App;
};

describe('App.stop', () => {
    beforeEach(() => vi.clearAllMocks());
    afterEach(() => vi.restoreAllMocks());

    it('calls stop on services in reverse registration order and closes the DB', async () => {
        const App = await loadApp();
        const app = new App([], { validate: false });
        const order: string[] = [];

        const mkSvc = (name: string, withStop = true) => ({
            run: vi.fn(),
            stop: withStop
                ? vi.fn().mockImplementation(async () => {
                      order.push(name);
                  })
                : undefined
        });

        (app as any).services.set('http', mkSvc('http'));
        (app as any).services.set('parser', mkSvc('parser'));
        (app as any).services.set('tg', mkSvc('tg'));
        (app as any).services.set('timetable', mkSvc('timetable', false));

        await app.stop();

        expect(order).toEqual(['tg', 'parser', 'http']);
    });

    it('continues when one service throws and still closes the DB', async () => {
        const App = await loadApp();
        const app = new App([], { validate: false });
        closeMock.mockClear();

        (app as any).services.set('http', {
            run: vi.fn(),
            stop: vi.fn().mockResolvedValue(undefined)
        });
        (app as any).services.set('tg', {
            run: vi.fn(),
            stop: vi.fn().mockRejectedValue(new Error('boom'))
        });

        await app.stop();

        expect((app as any).services.get('http').stop).toHaveBeenCalled();
        expect(closeMock).toHaveBeenCalledTimes(1);
    });

    it('enforces a per-service timeout and moves on to the next', async () => {
        const App = await loadApp();
        const app = new App([], { validate: false });

        let resolveStuck: () => void = () => {};
        (app as any).services.set('stuck', {
            run: vi.fn(),
            stop: vi.fn().mockImplementation(
                () =>
                    new Promise<void>((resolve) => {
                        resolveStuck = resolve;
                    })
            )
        });
        const fast = { run: vi.fn(), stop: vi.fn().mockResolvedValue(undefined) };
        (app as any).services.set('fast', fast);

        const done = app.stop({ timeoutMs: 50 });
        await done;
        resolveStuck();

        expect(fast.stop).toHaveBeenCalled();
    });
});
