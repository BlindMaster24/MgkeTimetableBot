import { sequelize } from './db';
import { runMigrations } from './db/migrator';
import { HttpService } from './http';
import { Logger } from './logger';
import { AliceApp } from './services/alice';
import { Api } from './services/api';
import { BotService } from './services/bots';
import { TgBot } from './services/bots/tg';
import { ViberBot } from './services/bots/viber';
import { VkBot } from './services/bots/vk';
import { GoogleService } from './services/google';
import { ImageService } from './services/image';
import { ParserService } from './services/parser';
import { topologicalSort, validateServiceDependencies } from './services/registry';
import { Timetable } from './services/timetable';
import { VKApp } from './services/vk_app';

export interface AppService {
    run(): Promise<any> | any;
    stop?(): Promise<any> | any;
}

const services = {
    http: HttpService,
    timetable: Timetable,
    bot: BotService,
    vk: VkBot,
    tg: TgBot,
    viber: ViberBot,
    image: ImageService,
    vkApp: VKApp,
    api: Api,
    alice: AliceApp,
    parser: ParserService,
    google_calendar: GoogleService
} as const;

type Services = typeof services;
export type AppServiceName = keyof Services;

export class App {
    public logger: Logger = new Logger('CORE');

    private services: Map<AppServiceName, AppService> = new Map();
    private init: boolean = false;

    constructor(initialServices: AppServiceName[] = [], options: { validate?: boolean } = {}) {
        if (initialServices.length === 0) return;
        const shouldValidate = options.validate ?? true;
        if (shouldValidate) {
            validateServiceDependencies(initialServices);
            const ordered = topologicalSort(initialServices);
            for (const service of ordered) {
                this.registerService(service);
            }
            return;
        }
        for (const service of initialServices) {
            this.registerService(service);
        }
    }

    public registerService(service: AppServiceName): void {
        const classHandler = services[service];
        const handler = new classHandler(this);

        this.services.set(service, handler);

        if (this.init) {
            handler.run();
        }
    }

    public getService<TServiceName extends AppServiceName & string, TService = InstanceType<Services[TServiceName]>>(
        service: TServiceName
    ): TService {
        const serviceInstance = this.services.get(service);

        if (!serviceInstance) {
            throw new Error(`Service '${String(service)}' not registered`);
        }

        return serviceInstance as TService;
    }

    public isServiceRegistered(service: AppServiceName): boolean {
        return this.services.has(service);
    }

    public async runServices(): Promise<void> {
        this.logger.log('Запуск...');

        const promises: Promise<any>[] = [];

        this.init = true;

        this.logger.log('Подключение к БД...');
        await sequelize.sync();
        const applied = await runMigrations();
        if (applied.length > 0) {
            this.logger.log('Применены миграции:', applied.join(', '));
        }
        this.logger.log('Подключение к БД: Успешно!');

        for (const [, service] of this.services) {
            promises.push(service.run());
        }

        await Promise.all(promises);

        this.logger.log('Проект успешно запущен');
        this.logger.log('Загруженные сервисы:', this.getServiceList().join(', '));
    }

    public getServiceList(): Array<string> {
        return Array.from(this.services.keys());
    }

    public async stop(options: { timeoutMs?: number } = {}): Promise<void> {
        const timeoutMs = options.timeoutMs ?? 15_000;
        this.logger.log('Остановка...');

        const order = Array.from(this.services.entries()).reverse();
        for (const [name, service] of order) {
            if (typeof service.stop !== 'function') continue;
            let timer: NodeJS.Timeout | undefined;
            try {
                await Promise.race([
                    Promise.resolve(service.stop()),
                    new Promise<void>((_resolve, reject) => {
                        timer = setTimeout(
                            () => reject(new Error(`stop timeout after ${timeoutMs}ms`)),
                            timeoutMs
                        );
                    })
                ]);
                this.logger.log(`Остановлено: ${name}`);
            } catch (error) {
                this.logger.error('service_stop_failed', { service: name, error });
            } finally {
                if (timer) clearTimeout(timer);
            }
        }

        try {
            await sequelize.close();
            this.logger.log('БД отключена');
        } catch (error) {
            this.logger.error('db_close_failed', { error });
        }
    }
}
