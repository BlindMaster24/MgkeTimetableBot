export function prepareError(error: Error): string | undefined {
    const stack = error.stack?.replaceAll(process.cwd(), '');
    const context = (error as any)?.parserContext;
    if (!context || typeof context !== 'object') {
        return stack;
    }

    const contextText = JSON.stringify(context, null, 2);
    if (!contextText || contextText === '{}') {
        return stack;
    }

    return [stack, 'context:', contextText].filter(Boolean).join('\n');
}
