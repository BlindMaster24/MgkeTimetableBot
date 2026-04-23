import { describe, it, expect } from 'vitest';
import IcsCommand from '../../src/services/bots/commands/rasp/ics';
import SettingsCommand from '../../src/services/bots/commands/settings/settings';
import DiffSettingsCommand from '../../src/services/bots/commands/settings/diff/index';
import SubscriptionsCommand from '../../src/services/bots/commands/settings/subscriptions';

type RegexpLike = RegExp | Record<string, RegExp>;

function match(regexp: RegexpLike, text: string): boolean {
    if (regexp instanceof RegExp) {
        return regexp.test(text);
    }

    return Object.values(regexp).some((r) => r.test(text));
}

describe('telegram command regexp flows', () => {
    it('ics accepts both button text and slash variants', () => {
        const cmd = new IcsCommand({} as any);

        for (const positive of ['/ics', 'ics', 'ICS']) {
            expect(match(cmd.regexp, positive), positive).toBe(true);
        }

        for (const negative of ['/ic', 'calendar']) {
            expect(match(cmd.regexp, negative), negative).toBe(false);
        }
    });

    it('settings accepts /settings but rejects misspellings', () => {
        const cmd = new SettingsCommand({} as any);

        expect(match(cmd.regexp, '/settings')).toBe(true);
        expect(match(cmd.regexp, '/settingz')).toBe(false);
        expect(match(cmd.regexp, '/setting')).toBe(false);
    });

    it('diff settings accept /diff and /diffsettings', () => {
        const cmd = new DiffSettingsCommand({} as any);

        expect(match(cmd.regexp, '/diff')).toBe(true);
        expect(match(cmd.regexp, '/diffsettings')).toBe(true);
        expect(match(cmd.regexp, '/abc')).toBe(false);
    });

    it('subscriptions accepts /subscriptions', () => {
        const cmd = new SubscriptionsCommand({} as any);

        expect(match(cmd.regexp, '/subscriptions')).toBe(true);
        expect(match(cmd.regexp, '/subscr')).toBe(false);
    });
});
