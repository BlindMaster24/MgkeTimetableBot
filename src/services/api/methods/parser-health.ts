import { config } from "../../../../config";
import { raspCache } from "../../parser";
import ApiDefaultMethod from "./_default";
import { promises as fs } from "fs";

export default class ParserHealthMethod extends ApiDefaultMethod {
    public readonly method = "parser-health";
    public readonly httpMethod = "GET";

    public async handler() {
        const metricsDir = config.parser.v2?.metrics?.dir || './cache/rasp/metrics';
        const readMetrics = async (name: string) => {
            try {
                const raw = await fs.readFile(`${metricsDir}/${name}.json`, 'utf8');
                return JSON.parse(raw);
            } catch {
                return null;
            }
        };

        const [studentMetrics, teacherMetrics] = await Promise.all([
            readMetrics('student'),
            readMetrics('teacher')
        ]);

        return {
            ok: Boolean(raspCache.successUpdate),
            lastSuccessUpdate: raspCache.successUpdate ? raspCache.groups.update || raspCache.teachers.update : 0,
            groups: {
                update: raspCache.groups.update,
                changed: raspCache.groups.changed,
                hash: raspCache.groups.hash
            },
            teachers: {
                update: raspCache.teachers.update,
                changed: raspCache.teachers.changed,
                hash: raspCache.teachers.hash
            },
            metrics: {
                student: studentMetrics,
                teacher: teacherMetrics
            }
        };
    }
}
