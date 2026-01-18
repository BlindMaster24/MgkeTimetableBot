import { Options } from "sequelize";
import { AppServiceName } from "./src/app";
import { GroupLessonExplain, TeacherLessonExplain } from "./src/services/timetable";

export type DayCall = [[string, string], [string, string]];
export type DayCallShort = [string, string];

export type ConfigScheme = {
    dev: boolean,
    services: AppServiceName[],
    db: Options,
    http: {
        servername: string,
        port: number
    },
    vk: {
        app: {
            id: number,
            secret: string,
            url: string
        },
        bot: {
            id: number,
            access_token: string,
            noticer: boolean
        },
        admin_ids: number[],
    },
    telegram: {
        token: string,
        admin_ids: number[],
        noticer: boolean
    },
    viber: {
        name: string,
        token: string,
        url: string,
        admin_ids: string[],
        noticer: boolean
    },
    // apk: {},
    api: {
        url: string
    },
    alice: {},
    google: {
        redirectDomain: string,
        url: string,
        oauth: {
            clientId: string,
            clientSecret: string
        },
        service_account: {
            clientEmail: string,
            privateKey: string
        },
        calendar_owners: string[],
        rateLimitter: {
            maxRequestsPerInterval: number,
            interval: number
        }
    },
    accept: {
        room: boolean,
        private: boolean,
        app: boolean
    },
    parser: {
        enabled: boolean,
        syncMode: boolean,
        localMode: boolean,
        ignoreHash: boolean,
        v2?: {
            enabled: boolean,
            fallbackToV1: boolean,
            weekPolicy: 'preferCurrent' | 'current' | 'closest',
            allowTwoTables: boolean,
            strict: boolean,
            diffLog: boolean,
            diffLogLimit: number,
            headerScanRows: number,
            minDaysInTable: number,
            maxLessonsPerDay: number,
            validationSample: number,
            hashMode: 'content' | 'tables',
            rawHtml: {
                enabled: boolean,
                dir: string,
                maxDays: number,
                replayPath: string | null,
                diffMaxLines: number,
                storeDaily: boolean
            },
            fetchRetry: number,
            weekJumpThreshold: number,
            sundayHoldCurrent: boolean,
            quarantine: {
                enabled: boolean,
                minLessons: number
            },
            metrics: {
                enabled: boolean,
                dir: string
            }
        },
        end_hour: number,
        activity: [number, number],
        update_interval: {
            default: number,
            activity: number,
            error: number,
            teams: number
        },
        alertableIgnoreFilter: {
            group: Pick<GroupLessonExplain, 'lesson' | 'type'>[],
            teacher: Pick<TeacherLessonExplain, 'lesson' | 'type'>[]
        },
        lessonIndexIfEmpty: number,
        endpoints: {
            timetableGroup: string
            timetableTeacher: string
            team: string[]
        },
        proxy: string | null
    },
    timetable: {
        weekdays: DayCall[],
        saturday: DayCall[],
        shortened_1h: DayCallShort[]
    },
    encrypt_key: Buffer,
    globalNoticer: boolean,
    globalAdblock: boolean
}
