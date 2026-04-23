import { fromError } from 'zod-validation-error';
import { configSchema } from './schema';

export function validateConfig(value: unknown): void {
    const result = configSchema.safeParse(value);
    if (result.success) {
        return;
    }

    const pretty = fromError(result.error, {
        prefix: 'Invalid config',
        prefixSeparator: ': ',
        issueSeparator: '\n  - ',
        unionSeparator: '\n    or '
    });

    // eslint-disable-next-line no-console
    console.error(`[config] ${pretty.toString()}`);
    throw new Error(pretty.toString());
}
