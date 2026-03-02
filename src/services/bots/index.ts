import { watch } from "chokidar";
import { existsSync, readdirSync, statSync } from "fs";
import path from "path";
import type { TelegramBotCommand } from "puregram/generated";
import { config } from "../../../config";
import { App, AppService } from "../../app";
import { Logger } from "../../logger";
import { ParsedPayload } from "../../utils";
import { AbstractCallback, AbstractCommand, AbstractCommandContext } from "./abstract";
import { BotChat } from "./chat";
import { BotCron } from "./cron";
import { BotEventController } from "./events/controller";
import "./subscriptions/model";

const cmdRootPath = path.join(__dirname, 'commands');
const cbRootPath = path.join(__dirname, 'callbacks');

type LoadedInstance<T> = {
    id: string,
    path: string,
    instance: T
}

export class BotService implements AppService {
    private readonly logger = new Logger('BOTS');
    public commands: {
        [id: string]: LoadedInstance<AbstractCommand>
    } = {};

    public callbacks: {
        [id: string]: LoadedInstance<AbstractCallback>
    } = {};

    public events: BotEventController;
    public cron: BotCron;
    private _init: Promise<void> | undefined;

    constructor(private app: App) {
        this.events = new BotEventController(app);
        this.cron = new BotCron(app);
    }

    public run() {
        this.events.run();
        this.cron.run();
    }

    // public getCommandById(id: string): AbstractCommand {
    //     const cmd = this.commands[id];

    //     if (!cmd) throw new Error(`Command with id '${id}' not found`);

    //     return cmd.instance;
    // }

    public getCallbackById<T extends AbstractCallback>(id: string): T {
        const cb = this.callbacks[id];
        if (!cb) {
            throw new Error(`Callback with id '${id}' not found`);
        }

        return cb.instance as T;
    }

    public searchCommandByMessage(message?: string, scene?: string | null): { regexp: string, cmd: AbstractCommand } | null {
        if (!message) return null

        const cmds = this.commands;

        for (const id in cmds) {
            const cmd = cmds[id].instance;

            if (scene !== undefined && cmd.scene !== undefined) {
                if (scene !== cmd.scene) continue;
            }

            if (cmd.regexp == null) continue;

            let matched: string | undefined;

            if (cmd.regexp instanceof RegExp) {
                if (cmd.regexp.test(message)) {
                    matched = 'index';
                }
            } else if (typeof cmd.regexp === 'object') {
                for (const regexId in cmd.regexp) {
                    const regex = cmd.regexp[regexId];

                    if (regex.test(message)) {
                        matched = regexId;
                        break;
                    }
                }
            }

            if (matched) {
                return {
                    regexp: matched,
                    cmd: cmd
                };
            }
        }

        return null;
    }

    public searchCommandByPayload(payload?: ParsedPayload): AbstractCommand | null {
        if (!payload) return null;

        const cmds = this.commands;

        for (const id in cmds) {
            const cmd = cmds[id].instance;

            if (cmd.payloadAction == null || cmd.payloadAction != payload.action) continue;

            return cmd;
        }

        return null;
    }

    public getCommand(context: AbstractCommandContext, chat: BotChat): { regexp?: string, cmd: AbstractCommand } | null {
        if (context.parsedPayload) {
            const cmd = this.searchCommandByPayload(context.parsedPayload);

            if (cmd) {
                return { cmd };
            }
        }

        const cmd = this.searchCommandByMessage(context.text, chat.scene);
        if (cmd) {
            return cmd;
        }

        return null;
    }

    // public getCallbackById(id: string): AbstractCallback {
    //     const cb = this.callbacks[id];

    //     if (!cb) throw new Error(`Command with id '${id}' not found`);

    //     return cb.instance;
    // }

    public getCallbackByPayload(payload?: ParsedPayload): AbstractCallback | null {
        if (!payload) return null;

        const cbs = this.callbacks;

        for (const id in cbs) {
            const cb = cbs[id].instance;

            if (cb.payloadAction != payload.action) continue;

            return cb;
        }

        return null;
    }

    public getBotCommands(showAdminCommands: boolean = false): TelegramBotCommand[] {
        return Object.values(this.commands).reduce<TelegramBotCommand[]>((commands, { instance }) => {
            const tgCommands = Array.isArray(instance.tgCommand) ? instance.tgCommand : [instance.tgCommand];

            for (const tgCommand of tgCommands) {
                if (tgCommand && (!instance.adminOnly || showAdminCommands)) {
                    tgCommand.command = tgCommand.command.toLowerCase();

                    if (instance.adminOnly) {
                        tgCommand.description = '[Ð°Ð´Ð¼] ' + tgCommand.description;
                    }

                    commands.push(tgCommand);
                }
            }

            return commands;
        }, [])
    }

    public init() {
        if (!this._init) {
            this._init = this.load();
        }

        return this._init;
    }

    private async load() {
        this.logger.info('start_loading_commands');

        await Promise.all([
            (async () => {
                const promises = this.loadFromDirectory(AbstractCommand, cmdRootPath);
                await Promise.all(promises);

                this.logger.info('commands_loaded', { count: Object.keys(this.commands).length });

                this.initFolderWatcher(cmdRootPath, AbstractCommand)
            })(),
            (async () => {
                const promises = this.loadFromDirectory(AbstractCallback, cbRootPath);
                await Promise.all(promises);

                this.logger.info('callbacks_loaded', { count: Object.keys(this.callbacks).length });

                this.initFolderWatcher(cbRootPath, AbstractCallback);
            })()
        ]);
    }

    private loadFromDirectory(classType: typeof AbstractCommand | typeof AbstractCallback, dir: string, rootPath?: string) {
        const promises: Promise<void>[] = [];

        if (!rootPath) {
            if (!existsSync(dir)) {
                return promises;
            }

            rootPath = dir;
        }

        const files = readdirSync(dir);
        for (const file of files) {
            const filePath: string = path.join(dir, file);

            if (statSync(filePath).isDirectory()) {
                const _promises = this.loadFromDirectory(classType, filePath, rootPath);
                promises.push(..._promises);

                continue;
            }

            promises.push(this.loadClass(classType, rootPath, filePath));
        }

        return promises;
    }

    public async loadClass(classType: typeof AbstractCommand | typeof AbstractCallback, rootPath: string, filePath: string) {
        if (!filePath.endsWith('.ts') && !filePath.endsWith('.js')) {
            return;
        }

        if (!existsSync(filePath)) {
            return;
        }

        const { default: cmdClass } = await import(filePath);
        if (cmdClass == undefined) return;

        const cmd = new cmdClass(this.app);
        if (!(cmd instanceof classType)) {
            throw new Error(`incorrect command class: ${filePath}`);
        }

        const id = this.pathToId(rootPath, filePath);
        cmd.id = id;

        if (cmd.requireServices) {
            for (const service of cmd.requireServices) {
                if (!this.app.isServiceRegistered(service)) {
                    return;
                }
            }
        }

        if (cmd instanceof AbstractCommand) {
            this.commands[id] = {
                id: id,
                path: filePath,
                instance: cmd
            };
        }

        if (cmd instanceof AbstractCallback) {
            this.callbacks[id] = {
                id: id,
                path: filePath,
                instance: cmd
            };
        }
    }

    public unloadCommand(cmd: LoadedInstance<AbstractCommand>) {
        delete require.cache[cmd.path];
        delete this.commands[cmd.id];
    }

    public unloadCallback(cb: LoadedInstance<AbstractCallback>) {
        delete require.cache[cb.path];
        delete this.callbacks[cb.id];
    }

    public async reloadCommandById(id: string, path?: string) {
        const cmd = this.commands[id];
        if (cmd) {
            path = cmd.path;
            this.unloadCommand(cmd);
        }

        if (!path) {
            throw new Error('Could\'t reload unloaded command by id without path')
        }

        await this.loadClass(AbstractCommand, cmdRootPath, path);
    }

    public async reloadCallbackById(id: string, path?: string) {
        const cb = this.callbacks[id];
        if (cb) {
            path = cb.path;
            this.unloadCallback(cb);
        }

        if (!path) {
            throw new Error('Could\'t reload unloaded callback by id without path')
        }

        await this.loadClass(AbstractCallback, cbRootPath, path);
    }

    private pathToId(rootPath: string, filePath: string) {
        return filePath
            .replace(rootPath, '')
            .replace(/^(\\|\/)/i, '')
            .replace(/(\.(ts|js))$/i, '')
            .replace(/\\|\//ig, '_');
    }

    private initFolderWatcher(folderPath: string, classType: typeof AbstractCommand | typeof AbstractCallback) {
        if (!config.dev) return;
        const className = classType.name.replace(/^Abstract/, '').toLowerCase();

        watch(folderPath)
            .on('ready', () => {
                this.logger.debug('watching_changes', { className, folderPath });
            })
            .on('change', async (path) => {
                const id = this.pathToId(folderPath, path);

                this.logger.info('detected_change_reloading', { className, id, path });
                try {
                    if (classType === AbstractCommand) {
                        await this.reloadCommandById(id, path);
                    } else if (classType === AbstractCallback) {
                        await this.reloadCallbackById(id, path);
                    } else throw new Error('unknown class type')
                } catch (e) {
                    this.logger.error('reload_failed', { className, id, path, error: e });

                    return;
                }

                this.logger.info('reload_success', { className, id });
            })
            .on('unlink', (path) => {
                const id = this.pathToId(folderPath, path);

                try {
                    if (classType === AbstractCommand) {
                        const cmd = this.commands[id];
                        if (!cmd) {
                            return;
                        }

                        this.logger.info('detected_unlink_unloading', { className, id, path });
                        this.unloadCommand(cmd);
                    } else if (classType === AbstractCallback) {
                        const cb = this.callbacks[id];
                        if (!cb) {
                            return;
                        }

                        this.logger.info('detected_unlink_unloading', { className, id, path });
                        this.unloadCallback(cb);
                    } else throw new Error('unknown class type')
                } catch (e) {
                    this.logger.error('unload_failed', { className, id, path, error: e });

                    return;
                }

                this.logger.info('unload_success', { className, id });
            })
    }
}

