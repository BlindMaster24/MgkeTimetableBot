import { existsSync, renameSync, statSync } from 'fs';
import { appendFile } from 'fs/promises';

function rotate(path: string, maxFiles: number): void {
    if (maxFiles < 1) {
        return;
    }

    for (let i = maxFiles - 1; i >= 1; i--) {
        const src = `${path}.${i}`;
        const dst = `${path}.${i + 1}`;
        if (existsSync(src)) {
            renameSync(src, dst);
        }
    }

    if (existsSync(path)) {
        renameSync(path, `${path}.1`);
    }
}

export async function writeFileLine(path: string, line: string, maxSizeMb: number, maxFiles: number): Promise<void> {
    const limit = Math.max(1, maxSizeMb) * 1024 * 1024;

    if (existsSync(path)) {
        const size = statSync(path).size;
        if (size >= limit) {
            rotate(path, Math.max(1, maxFiles));
        }
    }

    await appendFile(path, `${line}\n`, { encoding: 'utf8' });
}
