import { Request, Response } from 'express';
import rateLimit, { ipKeyGenerator } from 'express-rate-limit';
import { readdirSync } from 'fs';
import path from 'path';
import { StatusCode } from 'status-code-enum';
import { ZodError } from 'zod';
import { fromZodError } from 'zod-validation-error';
import { config } from '../../../config';
import { App, AppService } from '../../app';
import { newTraceId, runWithLogContext } from '../../logging';
import { Logger } from '../../logger';
import { ApiKeyModel } from './key';
import ApiDefaultMethod, { HandlerParams } from './methods/_default';

export type ApiRateLimitConfig = {
    enabled: boolean;
    windowMs: number;
    max: number;
    trustProxy: boolean;
};

export const createApiRateLimiter = (rl: Pick<ApiRateLimitConfig, 'windowMs' | 'max'>) =>
    rateLimit({
        windowMs: rl.windowMs,
        limit: rl.max,
        standardHeaders: 'draft-7',
        legacyHeaders: false,
        keyGenerator: (request) => {
            const token = request.header('authorization')?.split(' ')[1];
            if (token) return `key:${token}`;
            return `ip:${ipKeyGenerator(request.ip ?? '')}`;
        },
        handler: (_request, response) => {
            response.status(StatusCode.ClientErrorTooManyRequests).send({
                error: 'Превышен лимит запросов'
            });
        }
    });

export class Api implements AppService {
    private readonly logger = new Logger('API');
    private loaded: {
        [method: string]: {
            [method: string]: ApiDefaultMethod;
        };
    } = {};

    private requests: {
        [token: string]: {
            [reqId: string]: {
                time: number;
                timeout: NodeJS.Timeout;
            };
        };
    } = {};

    private app: App;

    constructor(app: App) {
        this.app = app;
    }

    public run() {
        const server = this.app.getService('http').getServer();

        this.loadMethods();

        const rl = {
            enabled: config.api.rateLimit?.enabled ?? true,
            windowMs: config.api.rateLimit?.windowMs ?? 60_000,
            max: config.api.rateLimit?.max ?? 120
        };
        if (rl.enabled) {
            server.use(`${config.api.url}/:method`, createApiRateLimiter(rl));
        }

        server.use(`${config.api.url}/:method`, (request, response) => {
            const requestId = request.header('x-request-id') || newTraceId();
            return runWithLogContext(
                {
                    traceId: requestId,
                    requestId,
                    service: 'api',
                    event: 'http_request',
                    path: request.path,
                    method: request.method
                },
                () => this.handler(request, response)
            );
        });
    }

    private getLastRequests(key: string, time: number): number {
        const currentTime: number = Date.now();

        const requests = this.requests[key];
        if (requests === undefined) return 0;

        let count = 0;
        for (const reqId in requests) {
            const req = requests[reqId];
            if (req.time >= currentTime - time) count++;
        }

        return count;
    }

    private checkLimit(key: ApiKeyModel) {
        if (key.limitPerSec === 0) return true;

        const id = key.id.toString();

        const limit = this.getLastRequests(id, 1e3) + 1;

        if (limit > key.limitPerSec) {
            return false;
        }

        return true;
    }

    private async applyRequest(key: ApiKeyModel, time: number) {
        const id = key.id.toString();

        if (this.requests[id] === undefined) {
            this.requests[id] = {};
        }

        const reqId = `${time}-${Math.random()}`;

        this.requests[id][reqId] = {
            time: time,
            timeout: setTimeout(() => {
                delete this.requests[id][reqId];

                if (Object.keys(this.requests[id]).length === 0) {
                    delete this.requests[id];
                }
            }, 1e3)
        };

        await key.update({ lastUsed: new Date() });
    }

    public loadMethods() {
        const methodsPath = path.join(__dirname, 'methods');

        const files = readdirSync(methodsPath);

        for (const file of files) {
            const { default: _class } = require(path.join(methodsPath, file));
            if (_class === undefined) continue;

            const method: ApiDefaultMethod = new _class();
            if (method.method === undefined) continue;

            if (this.loaded[method.httpMethod]?.[method.method] !== undefined)
                throw new Error(`${method.httpMethod} /${method.method} is already loaded!`);

            if (this.loaded[method.httpMethod] === undefined) this.loaded[method.httpMethod] = {};
            this.loaded[method.httpMethod][method.method] = method;
        }

        this.logger.info('methods_loaded', { count: this.getCountMethods() });
    }

    public getCountMethods() {
        let count = 0;
        for (const httpMethod in this.loaded) {
            count += Object.keys(this.loaded[httpMethod]).length;
        }
        return count;
    }

    private async handler(request: Request<{ method: string }>, response: Response) {
        const app = this.app;

        const time = Date.now();
        if (
            request.method.toUpperCase() === 'POST' &&
            request.headers['content-type']?.split(';')[0] !== 'application/json'
        )
            return this.badRequest(request, response);

        const method = request.params.method;

        const _class = this.loaded[request.method.toUpperCase()]?.[method];
        if (_class === undefined) {
            return response.status(StatusCode.ClientErrorNotFound).send('Method not found');
        }

        let token = request.header('authorization');
        if (token) {
            token = token.split(' ')[1];
        }
        if (!token) {
            return this.notAuthorized({ app, request, response });
        }

        (async () => {
            const key = await ApiKeyModel.fromKeyString(token);
            if (!key) {
                return this.notAuthorized({ app, request, response });
            }

            if (!this.checkLimit(key)) {
                return this.tooManyRequests({ app, request, response });
            }

            await this.applyRequest(key, time);

            try {
                const message = await _class.handler({ app, request, response });

                if (message === undefined) return;

                response.send({
                    response: message
                });
            } catch (e) {
                if (e instanceof ZodError) {
                    return response.status(StatusCode.ClientErrorBadRequest).send({
                        error: fromZodError(e).toString()
                    });
                }

                response.status(StatusCode.ServerErrorInternal).send('server error');
                this.logger.error('handler_error', { error: e, method });
            }
        })();
    }

    private notAuthorized({ response }: HandlerParams) {
        return response.status(StatusCode.ClientErrorUnauthorized).send({
            error: 'Неверный ключ авторизации'
        });
    }

    private badRequest(request: Request<{ method: string }>, response: Response) {
        return response.status(StatusCode.ClientErrorBadRequest).send('Bad Request');
    }

    private tooManyRequests({ response }: HandlerParams) {
        return response.status(StatusCode.ClientErrorTooManyRequests).send({
            error: 'Превышен лимит запросов'
        });
    }
}
