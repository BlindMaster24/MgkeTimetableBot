import { config } from "../../../../../../config";
import { KeyboardColor } from "../../../abstract";
import { raspCache } from "../../../../parser";

const sourceLabel = (source: 'site' | 'manual' | 'config') => {
    if (source === 'site') return '\u0441\u0430\u0439\u0442';
    if (source === 'manual') return '\u0432\u0440\u0443\u0447\u043d\u0443\u044e';
    return '\u043a\u043e\u043d\u0444\u0438\u0433';
};

const withActive = (label: string, active: boolean) => active ? `\u2705 ${label}` : label;

export const buildCallsMenu = (keyboard: any, serviceChat: any) => {
    const builder = keyboard.getKeyboardBuilder('CallsSettings');
    builder.add({
        text: '\uD83D\uDCCA \u041F\u043E\u043A\u0430\u0437\u0430\u0442\u044C'
    }).row();

    if (serviceChat.isSuperAdmin()) {
        const activeSource = raspCache.calls.overrideSource ?? raspCache.calls.active.source;
        const autoActive = !raspCache.calls.overrideSource;

        builder.add({
            text: '\u2705 \u041E\u0431\u043D\u043E\u0432\u0438\u0442\u044C \u0441 \u0441\u0430\u0439\u0442\u0430'
        }).row().add({
            text: '\u270F\uFE0F \u0418\u0437\u043C\u0435\u043D\u0438\u0442\u044C \u0432\u0440\u0443\u0447\u043d\u0443\u044e'
        }).row().add({
            text: withActive('\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A: \u0441\u0430\u0439\u0442', activeSource === 'site')
        }).add({
            text: withActive('\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A: \u0432\u0440\u0443\u0447\u043d\u0443\u044e', activeSource === 'manual')
        }).row().add({
            text: withActive('\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A: \u043a\u043e\u043d\u0444\u0438\u0433', activeSource === 'config')
        }).add({
            text: withActive('\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A: \u0430\u0432\u0442\u043e', autoActive)
        }).row();
    }

    const lines: string[] = ['\u0423\u043F\u0440\u0430\u0432\u043B\u0435\u043D\u0438\u0435 \u0440\u0430\u0441\u043F\u0438\u0441\u0430\u043D\u0438\u0435\u043C \u0437\u0432\u043E\u043D\u043A\u043E\u0432.'];

    const activeSource = raspCache.calls.active.source;
    const override = raspCache.calls.overrideSource;
    if (override) {
        lines.push(`\u041F\u0435\u0440\u0435\u043E\u043F\u0440\u0435\u0434\u0435\u043B\u0435\u043D\u0438\u0435 \u0438\u0441\u0442\u043E\u0447\u043D\u0438\u043A\u0430: ${sourceLabel(override)}`);
    } else {
        lines.push(`\u0418\u0441\u0442\u043E\u0447\u043D\u0438\u043A: \u0430\u0432\u0442\u043E (\u0441\u0435\u0439\u0447\u0430\u0441: ${sourceLabel(activeSource)})`);
    }

    if (!override && activeSource !== 'site' && raspCache.calls.siteEmptyNotifiedAt) {
        lines.push('\u0421\u0430\u0439\u0442 \u043E\u0442\u0434\u0430\u043B \u043F\u0443\u0441\u0442\u043E.');
    }

    if (activeSource === 'site' && raspCache.calls.site.updatedAtRaw) {
        lines.push(`\u041E\u0431\u043D\u043E\u0432\u043B\u0435\u043D\u043E \u043D\u0430 \u0441\u0430\u0439\u0442\u0435: ${raspCache.calls.site.updatedAtRaw}`);
    } else if (activeSource === 'manual' && raspCache.calls.manualReason) {
        lines.push(`\u041F\u0440\u0438\u0447\u0438\u043D\u0430: ${raspCache.calls.manualReason}`);
    } else if (activeSource === 'config' && config.timetable.weekdays.length) {
        lines.push('\u0418\u0441\u043F\u043E\u043B\u044C\u0437\u0443\u044E\u0442\u0441\u044F \u0434\u0430\u043D\u043D\u044B\u0435 \u0438\u0437 \u043A\u043E\u043D\u0444\u0438\u0433\u0430');
    }

    const finalKeyboard = builder.add({
        text: '\u041C\u0435\u043D\u044E \u043D\u0430\u0441\u0442\u0440\u043E\u0435\u043A',
        color: KeyboardColor.SECONDARY_COLOR
    }).add({
        text: '\u0413\u043B\u0430\u0432\u043D\u043E\u0435 \u043C\u0435\u043D\u044E',
        color: KeyboardColor.SECONDARY_COLOR
    });

    return { text: lines.join('\n'), keyboard: finalKeyboard };
};
