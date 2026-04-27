import express, { Application, NextFunction, Request, Response } from 'express';
import { config } from '../config';
import { App, AppService } from './app';
import { newTraceId, runWithLogContext } from './logging';
import { Logger } from './logger';
import { raspCache } from './services/parser/raspCache';
import { getIp, getParams, replaceWithValueLength } from './utils';

type ErrorWithStatus = Error & Partial<{ status: number; statusCode: number; code: any; type: any }>;

export class HttpService implements AppService {
    public logger: Logger = new Logger('HTTP');

    private http: Application;

    public ignoreJsonParserUrls: string[] = [];

    private app: App;

    constructor(app: App) {
        this.app = app;
        this.http = express();
    }

    public getServer() {
        return this.http;
    }

    public run() {
        if (config.api.rateLimit?.trustProxy) {
            this.http.set('trust proxy', 1);
        }
        this.http.use(express.static('./public/'));
        this.http.use((req, res, next) => {
            const incoming = req.header('x-request-id');
            const requestId = typeof incoming === 'string' && incoming.trim().length > 0 ? incoming : newTraceId();
            res.setHeader('x-request-id', requestId);

            runWithLogContext(
                {
                    traceId: requestId,
                    requestId,
                    service: 'http',
                    event: 'http_request',
                    method: req.method,
                    path: req.path
                },
                () => {
                    next();
                }
            );
        });

        if (config.dev) {
            this.logRoutes();
        }

        this.setupOriginHeaders();
        this.registerHealthEndpoints();
        this.setupJsonBodyParser();

        this.http.use(this.errorHandler.bind(this));

        this.http.listen(config.http.port, () => {
            this.logger.log(`Сервер запущен на порту: ${config.http.port}`);
        });
    }

    private setupOriginHeaders() {
        this.http.use((req, res, next) => {
            res.header('Access-Control-Allow-Origin', '*');
            res.header('Access-Control-Allow-Methods', 'DELETE, POST, GET, OPTIONS');
            res.header('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept');
            res.header('Access-Control-Max-Age', '86400');
            next();
        });

        this.http.use((req, res, next) => {
            if (req.method.toUpperCase() !== 'OPTIONS') return next();

            res.send();
        });
    }

    private setupJsonBodyParser() {
        this.http.use((req, res, next) => {
            for (const url of this.ignoreJsonParserUrls) {
                if (req.path.startsWith(url)) {
                    return next();
                }
            }

            return express.json({})(req, res, next);
        });
    }

    private registerHealthEndpoints() {
        this.http.get('/api/health', (_req, res) => {
            res.json({
                ok: true,
                uptime: Math.floor(process.uptime()),
                services: this.app.getServiceList(),
                parserOk: Boolean(raspCache.successUpdate)
            });
        });

        this.http.get('/api/metrics', (_req, res) => {
            const nowSec = Math.floor(Date.now() / 1000);
            const memory = process.memoryUsage();
            const services = this.app.getServiceList();
            const parserOk = raspCache.successUpdate ? 1 : 0;
            const lastUpdate = raspCache.successUpdate ? raspCache.groups.update || raspCache.teachers.update : 0;

            const lines: string[] = [];
            lines.push('# HELP bot_up Whether the bot process is up (always 1 when scraped).');
            lines.push('# TYPE bot_up gauge');
            lines.push('bot_up 1');

            lines.push('# HELP bot_uptime_seconds Seconds since the bot process started.');
            lines.push('# TYPE bot_uptime_seconds gauge');
            lines.push(`bot_uptime_seconds ${Math.floor(process.uptime())}`);

            lines.push('# HELP bot_services_enabled Number of services enabled in this App instance.');
            lines.push('# TYPE bot_services_enabled gauge');
            lines.push(`bot_services_enabled ${services.length}`);

            lines.push('# HELP bot_service_enabled 1 if the named service is enabled, 0 otherwise.');
            lines.push('# TYPE bot_service_enabled gauge');
            for (const service of services) {
                lines.push(`bot_service_enabled{service="${service}"} 1`);
            }

            lines.push('# HELP bot_parser_ok 1 if the parser last update succeeded, 0 otherwise.');
            lines.push('# TYPE bot_parser_ok gauge');
            lines.push(`bot_parser_ok ${parserOk}`);

            lines.push(
                '# HELP bot_parser_last_update_timestamp_seconds Unix timestamp of the last successful parser update.'
            );
            lines.push('# TYPE bot_parser_last_update_timestamp_seconds gauge');
            lines.push(`bot_parser_last_update_timestamp_seconds ${Math.floor(lastUpdate / 1000)}`);

            lines.push('# HELP bot_parser_staleness_seconds Seconds since the last successful parser update.');
            lines.push('# TYPE bot_parser_staleness_seconds gauge');
            lines.push(`bot_parser_staleness_seconds ${lastUpdate ? nowSec - Math.floor(lastUpdate / 1000) : 0}`);

            lines.push('# HELP bot_process_memory_bytes Process memory usage in bytes.');
            lines.push('# TYPE bot_process_memory_bytes gauge');
            lines.push(`bot_process_memory_bytes{area="rss"} ${memory.rss}`);
            lines.push(`bot_process_memory_bytes{area="heap_total"} ${memory.heapTotal}`);
            lines.push(`bot_process_memory_bytes{area="heap_used"} ${memory.heapUsed}`);
            lines.push(`bot_process_memory_bytes{area="external"} ${memory.external}`);

            res.setHeader('content-type', 'text/plain; version=0.0.4; charset=utf-8');
            res.send(lines.join('\n') + '\n');
        });
    }

    private logRoutes() {
        this.http.use((req, res, next) => {
            this.logger.debug(getIp(req), req.path, replaceWithValueLength(getParams(req)));
            next();
        });
    }

    private errorHandler(err: ErrorWithStatus | null, req: Request, response: Response, next: NextFunction) {
        if (err === null) return next();

        let status: number = err.status ?? err.statusCode ?? 500;

        if (status < 400) {
            status = 500;
        }

        response.status(status);

        const body: any = {
            status
        };

        if (process.env.NODE_ENV !== 'production') {
            // body.stack = err.stack;
            body.trace =
                err.stack
                    ?.replace(/\ +/g, ' ')
                    .replace(/(\n\ )+/g, '\n')
                    .split('\t')
                    .join('')
                    .split('\n')
                    .slice(1) || [];
        }

        Object.assign(body, {
            code: err.code,
            name: err.name,
            message: err.message,
            type: err.type
        });

        if (status >= 500) {
            this.logger.error(err);
        }

        return response.json(body);
    }
}
