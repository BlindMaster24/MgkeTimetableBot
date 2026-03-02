import { InlineKeyboard as TgInlineKeyboardBuilder, Keyboard as TgKeyboardBuilder } from 'grammy';
import { ButtonType, KeyboardBuilder } from '../abstract';

export function convertAbstractToTg(aKeyboard?: KeyboardBuilder): TgKeyboardBuilder | TgInlineKeyboardBuilder | undefined {
    if (!aKeyboard) {
        return;
    }

    let keyboard: TgInlineKeyboardBuilder | TgKeyboardBuilder;
    if (aKeyboard.isInline) {
        keyboard = new TgInlineKeyboardBuilder();

        for (const row of aKeyboard.buttons) {
            for (const button of row) {
                if (button.type === ButtonType.Url) {
                    if (!button.url) {
                        continue;
                    }

                    keyboard.url(button.text, button.url);
                } else {
                    keyboard.text(button.text, button.payload);
                }
            }

            keyboard.row()
        }
    } else {
        keyboard = new TgKeyboardBuilder().resized();

        for (const row of aKeyboard.buttons) {
            for (const button of row) {
                keyboard.text(button.text);
            }

            keyboard.row()
        }
    }

    return keyboard
}
