import assert from 'assert';
import IcsCommand from '../src/services/bots/commands/rasp/ics';
import SettingsCommand from '../src/services/bots/commands/settings/settings';
import DiffSettingsCommand from '../src/services/bots/commands/settings/diff/index';
import SubscriptionsCommand from '../src/services/bots/commands/settings/subscriptions';

type Check = { name: string; regexp: RegExp | Record<string, RegExp>; positives: string[]; negatives?: string[] };

function match(regexp: RegExp | Record<string, RegExp>, text: string): boolean {
    if (regexp instanceof RegExp) {
        return regexp.test(text);
    }

    return Object.values(regexp).some((r) => r.test(text));
}

function run(check: Check) {
    for (const input of check.positives) {
        assert.strictEqual(match(check.regexp, input), true, `${check.name} should match: ${input}`);
    }

    for (const input of check.negatives ?? []) {
        assert.strictEqual(match(check.regexp, input), false, `${check.name} should not match: ${input}`);
    }
}

(() => {
    console.log('botTelegramFlowTest: start');
    const ics = new IcsCommand({} as any);
    const settings = new SettingsCommand({} as any);
    const diff = new DiffSettingsCommand({} as any);
    const subs = new SubscriptionsCommand({} as any);

    run({
        name: 'ics',
        regexp: ics.regexp,
        positives: ['/ics', 'ics', 'ICS'],
        negatives: ['/ic', 'calendar']
    });
    console.log('botTelegramFlowTest: ics-regexp ok');

    run({
        name: 'settings',
        regexp: settings.regexp,
        positives: ['/settings'],
        negatives: ['/settingz', '/setting']
    });
    console.log('botTelegramFlowTest: settings-regexp ok');

    run({
        name: 'diff',
        regexp: diff.regexp,
        positives: ['/diff', '/diffsettings'],
        negatives: ['/abc']
    });
    console.log('botTelegramFlowTest: diff-regexp ok');

    run({
        name: 'subscriptions',
        regexp: subs.regexp,
        positives: ['/subscriptions'],
        negatives: ['/subscr']
    });
    console.log('botTelegramFlowTest: subscriptions-regexp ok');

    console.log('botTelegramFlowTest: ok');
})();
