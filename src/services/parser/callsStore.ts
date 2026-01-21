import { StorageModel } from "../bots/storage/model";

export type CallsOverrideSource = 'site' | 'manual' | 'config';

const STORAGE_NAME = 'calls';
const OVERRIDE_KEY = 'override_source';
const REASON_KEY = 'manual_reason';
const REASON_UPDATED_AT_KEY = 'manual_reason_updated_at';

const getValue = async (key: string): Promise<string | null> => {
    const row = await StorageModel.findOne({
        where: {
            storage: STORAGE_NAME,
            key
        },
        rejectOnEmpty: false
    });

    return row?.value ?? null;
};

const setValue = async (key: string, value: string | null) => {
    if (!value) {
        await StorageModel.destroy({
            where: {
                storage: STORAGE_NAME,
                key
            }
        });
        return;
    }

    await StorageModel.upsert({
        storage: STORAGE_NAME,
        key,
        value,
        expiresAt: null
    });
};

export const getCallsOverrideSource = async (): Promise<CallsOverrideSource | null> => {
    const value = await getValue(OVERRIDE_KEY);
    if (value === 'site' || value === 'manual' || value === 'config') {
        return value;
    }
    return null;
};

export const setCallsOverrideSource = async (value: CallsOverrideSource | null): Promise<void> => {
    await setValue(OVERRIDE_KEY, value);
};

export const getCallsManualReason = async (): Promise<{ reason: string | null; updatedAt: number | null }> => {
    const reason = await getValue(REASON_KEY);
    const updatedRaw = await getValue(REASON_UPDATED_AT_KEY);
    const updatedAt = updatedRaw ? Number(updatedRaw) : null;
    return { reason, updatedAt };
};

export const setCallsManualReason = async (reason: string | null): Promise<void> => {
    if (!reason) {
        await setValue(REASON_KEY, null);
        await setValue(REASON_UPDATED_AT_KEY, null);
        return;
    }

    await setValue(REASON_KEY, reason);
    await setValue(REASON_UPDATED_AT_KEY, String(Date.now()));
};
