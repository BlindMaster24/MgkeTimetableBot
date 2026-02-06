import assert from 'assert';
import { redactContext, redactMessageText } from '../src/logging/redact';

function testSecretRedaction() {
    const data = {
        token: '123',
        nested: {
            privateKey: 'abc',
            keep: 'value'
        },
        arr: [{ password: 'x' }, { ok: 1 }]
    };

    const out = redactContext(data, 10) as any;

    assert.strictEqual(out.token, '[REDACTED]');
    assert.strictEqual(out.nested.privateKey, '[REDACTED]');
    assert.strictEqual(out.nested.keep, 'value');
    assert.strictEqual(out.arr[0].password, '[REDACTED]');
    assert.strictEqual(out.arr[1].ok, 1);
}

function testStringTrim() {
    const out = redactContext({ text: 'abcdefghijklmnopqrstuvwxyz' }, 5) as any;
    assert.strictEqual(out.text, 'abcde...(26)');
}

function testMessageTextFields() {
    const out = redactMessageText({ messageText: 'hello world', text: 'abcdef' }, 4) as any;

    assert.strictEqual(out.messageText, 'hell...(11)');
    assert.strictEqual(out.messageTextLength, 11);
    assert.strictEqual(out.text, 'abcd...(6)');
    assert.strictEqual(out.textLength, 6);
}

(() => {
    testSecretRedaction();
    testStringTrim();
    testMessageTextFields();
    console.log('loggingRedactionTest: ok');
})();
