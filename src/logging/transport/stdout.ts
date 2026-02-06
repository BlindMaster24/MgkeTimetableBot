export function writeStdout(line: string): void {
    process.stdout.write(`${line}\n`);
}
