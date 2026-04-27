import { describe, it, expect } from 'vitest';
import { closestJaroWinkler } from '../../src/utils/natural';

describe('closestJaroWinkler', () => {
    it('returns an exact hit with score 1 when the value is in the array', () => {
        const res = closestJaroWinkler('Иванов И.И.', ['Петров П.П.', 'Иванов И.И.']);
        expect(res).toEqual({ value: 'Иванов И.И.', score: 1 });
    });

    it('returns the closest candidate with its score when there is no exact match', () => {
        const res = closestJaroWinkler('Иванов И.И', ['Петров П.П.', 'Иванов И.И.']);

        expect(res).toBeDefined();
        expect(res!.value).toBe('Иванов И.И.');
        expect(res!.score).toBeGreaterThan(0.9);
    });

    it('returns undefined when the best score is below the threshold', () => {
        const res = closestJaroWinkler('absolute nonsense string', ['Иванов И.И.'], 0.9);
        expect(res).toBeUndefined();
    });
});
