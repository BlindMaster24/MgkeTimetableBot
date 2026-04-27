import { z } from 'zod';

const APP_SERVICE_NAMES = [
    'http',
    'timetable',
    'bot',
    'vk',
    'tg',
    'viber',
    'image',
    'vkApp',
    'api',
    'alice',
    'parser',
    'google_calendar'
] as const;

const loggingSchema = z
    .object({
        level: z.enum(['error', 'warn', 'info', 'debug']),
        format: z.enum(['json', 'text']),
        output: z.object({
            stdout: z.boolean(),
            file: z.object({
                enabled: z.boolean(),
                path: z.string().min(1),
                maxSizeMb: z.number().positive(),
                maxFiles: z.number().int().positive()
            })
        }),
        redact: z.object({
            messageText: z.boolean(),
            maxPreviewLen: z.number().int().nonnegative()
        })
    })
    .optional();

const dbSchema = z
    .object({
        dialect: z.string().min(1)
    })
    .passthrough();

const httpSchema = z.object({
    servername: z.string().min(1),
    port: z.number().int().min(1).max(65535)
});

const vkSchema = z.object({
    app: z.object({
        id: z.number().int(),
        secret: z.string(),
        url: z.string()
    }),
    bot: z.object({
        id: z.number().int(),
        access_token: z.string(),
        noticer: z.boolean()
    }),
    admin_ids: z.array(z.number().int())
});

const telegramSchema = z.object({
    token: z.string(),
    admin_ids: z.array(z.number().int()),
    noticer: z.boolean()
});

const viberSchema = z.object({
    name: z.string(),
    token: z.string(),
    url: z.string(),
    admin_ids: z.array(z.string()),
    noticer: z.boolean()
});

const apiSchema = z.object({
    url: z.string().min(1)
});

const googleSchema = z.object({
    redirectDomain: z.string(),
    url: z.string(),
    oauth: z.object({
        clientId: z.string(),
        clientSecret: z.string()
    }),
    service_account: z.object({
        clientEmail: z.string(),
        privateKey: z.string()
    }),
    calendar_owners: z.array(z.string()),
    rateLimitter: z.object({
        maxRequestsPerInterval: z.number().int().positive(),
        interval: z.number().int().positive()
    })
});

const calendarSchema = z.object({
    ics: z.object({
        enabled: z.boolean()
    })
});

const acceptSchema = z.object({
    room: z.boolean(),
    private: z.boolean(),
    app: z.boolean()
});

const parserV2Schema = z
    .object({
        enabled: z.boolean(),
        fallbackToV1: z.boolean(),
        weekPolicy: z.enum(['preferCurrent', 'current', 'closest']),
        allowTwoTables: z.boolean(),
        strict: z.boolean(),
        diffLog: z.boolean(),
        diffLogLimit: z.number().int().nonnegative(),
        headerScanRows: z.number().int().positive(),
        minDaysInTable: z.number().int().positive(),
        maxLessonsPerDay: z.number().int().positive(),
        validationSample: z.number().int().nonnegative(),
        hashMode: z.enum(['content', 'tables']),
        rawHtml: z.object({
            enabled: z.boolean(),
            dir: z.string(),
            maxDays: z.number().int().nonnegative(),
            replayPath: z.string().nullable(),
            diffMaxLines: z.number().int().nonnegative(),
            storeDaily: z.boolean()
        }),
        fetchRetry: z.number().int().nonnegative(),
        weekJumpThreshold: z.number().int().nonnegative(),
        sundayHoldCurrent: z.boolean(),
        quarantine: z.object({
            enabled: z.boolean(),
            minLessons: z.number().int().nonnegative()
        }),
        metrics: z.object({
            enabled: z.boolean(),
            dir: z.string()
        })
    })
    .optional();

const parserSchema = z.object({
    enabled: z.boolean(),
    syncMode: z.boolean(),
    localMode: z.boolean(),
    ignoreHash: z.boolean(),
    v2: parserV2Schema,
    end_hour: z.number().int().min(0).max(23),
    activity: z.tuple([z.number().int(), z.number().int()]),
    update_interval: z.object({
        default: z.number().positive(),
        activity: z.number().positive(),
        error: z.number().positive(),
        teams: z.number().positive(),
        calls: z.number().positive()
    }),
    alertableIgnoreFilter: z.object({
        group: z.array(z.any()),
        teacher: z.array(z.any())
    }),
    lessonIndexIfEmpty: z.number().int(),
    endpoints: z.object({
        timetableGroup: z.string().min(1),
        timetableTeacher: z.string().min(1),
        team: z.array(z.string()),
        bellSchedule: z.string().min(1)
    }),
    calls: z.object({
        enabled: z.boolean(),
        preferSite: z.boolean(),
        notify: z.boolean()
    }),
    proxy: z.string().nullable()
});

const dayCallSchema = z.tuple([z.tuple([z.string(), z.string()]), z.tuple([z.string(), z.string()])]);

const dayCallShortSchema = z.tuple([z.string(), z.string()]);

const timetableSchema = z.object({
    weekdays: z.array(dayCallSchema),
    saturday: z.array(dayCallSchema),
    shortened_1h: z.array(dayCallShortSchema)
});

export const configSchema = z
    .object({
        dev: z.boolean(),
        logging: loggingSchema,
        services: z.array(z.enum(APP_SERVICE_NAMES)),
        db: dbSchema,
        http: httpSchema,
        vk: vkSchema,
        telegram: telegramSchema,
        viber: viberSchema,
        api: apiSchema,
        alice: z.object({}).passthrough(),
        google: googleSchema,
        calendar: calendarSchema,
        accept: acceptSchema,
        parser: parserSchema,
        timetable: timetableSchema,
        encrypt_key: z.custom<Buffer>((val) => Buffer.isBuffer(val), {
            message: 'encrypt_key must be a Buffer'
        }),
        globalNoticer: z.boolean(),
        globalAdblock: z.boolean()
    })
    .passthrough()
    .superRefine((cfg, ctx) => {
        const services = cfg.services;
        const needStrong = (svc: string, path: (string | number)[], ok: boolean, msg: string) => {
            if (services.includes(svc as (typeof APP_SERVICE_NAMES)[number]) && !ok) {
                ctx.addIssue({ code: 'custom', path, message: msg });
            }
        };

        needStrong(
            'tg',
            ['telegram', 'token'],
            cfg.telegram.token.length >= 1,
            'telegram.token is required when "tg" service is enabled'
        );
        needStrong(
            'vk',
            ['vk', 'bot', 'access_token'],
            cfg.vk.bot.access_token.length >= 1,
            'vk.bot.access_token is required when "vk" service is enabled'
        );
        needStrong(
            'viber',
            ['viber', 'token'],
            cfg.viber.token.length >= 1,
            'viber.token is required when "viber" service is enabled'
        );

        const encryptKeyConsumers = ['api', 'vkApp', 'tg', 'vk', 'viber', 'image', 'google_calendar'] as const;
        const needsEncryptKey = services.some((s) => (encryptKeyConsumers as readonly string[]).includes(s as string));
        if (needsEncryptKey && cfg.encrypt_key.length === 0) {
            const enabled = services.filter((s) => (encryptKeyConsumers as readonly string[]).includes(s as string));
            ctx.addIssue({
                code: 'custom',
                path: ['encrypt_key'],
                message: `encrypt_key must be a non-empty Buffer when any of these services is enabled: ${enabled.join(', ')}`
            });
        }
    });

export type ValidatedConfig = z.infer<typeof configSchema>;
