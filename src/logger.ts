import { loggingEngine } from './logging';
import { getLogContext } from './logging/context';
import { LogContext } from './logging/types';

export class Logger {
    constructor(
        private loggerName: string,
        private context: LogContext = {}
    ) {}

    public extend(extendName: string): Logger {
        return new Logger(this.loggerName + ':' + extendName, this.context);
    }

    public withContext(context: LogContext): Logger {
        return new Logger(this.loggerName, { ...this.context, ...context });
    }

    public log(...message: any[]) {
        this.info(...message);
    }

    public debug(...message: any[]) {
        this.write('debug', message);
    }

    public info(...message: any[]) {
        this.write('info', message);
    }

    public warn(...message: any[]) {
        this.write('warn', message);
    }

    public error(...message: any[]) {
        this.write('error', message);
    }

    private write(level: 'error' | 'warn' | 'info' | 'debug', message: any[]) {
        const [first, ...rest] = message;
        const text = typeof first === 'string' ? first : 'log';
        const context: LogContext = { ...getLogContext(), ...this.context };

        if (typeof first !== 'string' && first !== undefined) {
            context.data = first;
        }

        if (rest.length) {
            context.args = rest;
        }

        loggingEngine
            .write(level, this.loggerName, text, Object.keys(context).length ? context : undefined)
            .catch(() => {});
    }
}
