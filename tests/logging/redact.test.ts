import { describe, it, expect } from 'vitest';
import { redactContext, redactMessageText } from '../../src/logging/redact';

describe('logging redaction', () => {
    it('redacts known secret keys recursively', () => {
        const data = {
            token: '123',
            nested: {
                privateKey: 'abc',
                keep: 'value'
            },
            arr: [{ password: 'x' }, { ok: 1 }]
        };

        const out = redactContext(data, 10) as any;

        expect(out.token).toBe('[REDACTED]');
        expect(out.nested.privateKey).toBe('[REDACTED]');
        expect(out.nested.keep).toBe('value');
        expect(out.arr[0].password).toBe('[REDACTED]');
        expect(out.arr[1].ok).toBe(1);
    });

    it('trims long strings and annotates with original length', () => {
        const out = redactContext({ text: 'abcdefghijklmnopqrstuvwxyz' }, 5) as any;
        expect(out.text).toBe('abcde...(26)');
    });

    it('handles messageText and text fields with explicit length metadata', () => {
        const out = redactMessageText({ messageText: 'hello world', text: 'abcdef' }, 4) as any;

        expect(out.messageText).toBe('hell...(11)');
        expect(out.messageTextLength).toBe(11);
        expect(out.text).toBe('abcd...(6)');
        expect(out.textLength).toBe(6);
    });
});
