import { describe, expect, it } from 'vitest';
import {
    ServiceCycleError,
    ServiceDependencyError,
    serviceDependencies,
    topologicalSort,
    validateServiceDependencies
} from '../../src/services/registry';

describe('validateServiceDependencies', () => {
    it('passes for empty service list', () => {
        expect(() => validateServiceDependencies([])).not.toThrow();
    });

    it('passes when all required deps are present', () => {
        expect(() => validateServiceDependencies(['http', 'api', 'timetable', 'parser'])).not.toThrow();
    });

    it('fails when required dep is missing', () => {
        expect(() => validateServiceDependencies(['api'])).toThrow(ServiceDependencyError);
    });

    it('reports every missing dep for clear diagnostics', () => {
        try {
            validateServiceDependencies(['api', 'alice', 'tg', 'viber']);
            throw new Error('should have thrown');
        } catch (err) {
            expect(err).toBeInstanceOf(ServiceDependencyError);
            const e = err as ServiceDependencyError;
            expect(e.missing).toEqual([
                { service: 'api', requires: 'http' },
                { service: 'alice', requires: 'http' },
                { service: 'tg', requires: 'bot' },
                { service: 'viber', requires: 'bot' },
                { service: 'viber', requires: 'http' }
            ]);
            expect(e.message).toMatch(/requires 'http'/);
            expect(e.message).toMatch(/requires 'bot'/);
        }
    });

    it('ignores optional deps that are missing', () => {
        expect(() => validateServiceDependencies(['timetable'])).not.toThrow();
        expect(() => validateServiceDependencies(['api', 'http'])).not.toThrow();
    });
});

describe('topologicalSort', () => {
    it('returns an empty order for empty input', () => {
        expect(topologicalSort([])).toEqual([]);
    });

    it('places dependencies before dependents', () => {
        const order = topologicalSort(['api', 'http']);
        expect(order.indexOf('http')).toBeLessThan(order.indexOf('api'));
    });

    it('respects optional dependencies in startup order', () => {
        const order = topologicalSort(['timetable', 'parser']);
        expect(order).toEqual(['parser', 'timetable']);
    });

    it('orders a realistic production set correctly', () => {
        const order = topologicalSort(['http', 'parser', 'timetable', 'bot', 'tg', 'image', 'api', 'google_calendar']);
        expect(order.indexOf('http')).toBeLessThan(order.indexOf('api'));
        expect(order.indexOf('bot')).toBeLessThan(order.indexOf('tg'));
        expect(order.indexOf('bot')).toBeLessThan(order.indexOf('google_calendar'));
        expect(order.indexOf('http')).toBeLessThan(order.indexOf('google_calendar'));
        expect(order.indexOf('parser')).toBeLessThan(order.indexOf('timetable'));
        expect(order.indexOf('image')).toBeLessThan(order.indexOf('tg'));
    });

    it('always places required deps before dependents regardless of input order', () => {
        const a = topologicalSort(['http', 'api', 'timetable']);
        const b = topologicalSort(['api', 'timetable', 'http']);
        expect(a.indexOf('http')).toBeLessThan(a.indexOf('api'));
        expect(b.indexOf('http')).toBeLessThan(b.indexOf('api'));
    });
});

describe('service registry declarations', () => {
    it('declares every service exactly once', () => {
        const keys = Object.keys(serviceDependencies);
        expect(new Set(keys).size).toBe(keys.length);
    });

    it('never declares a service as its own dependency', () => {
        for (const [name, spec] of Object.entries(serviceDependencies)) {
            expect(spec.required).not.toContain(name);
            expect(spec.optional).not.toContain(name);
        }
    });

    it('does not declare the same dep as both required and optional', () => {
        for (const [, spec] of Object.entries(serviceDependencies)) {
            const required = new Set(spec.required);
            for (const dep of spec.optional) {
                expect(required.has(dep)).toBe(false);
            }
        }
    });
});

describe('ServiceCycleError', () => {
    it('is a thrown type', () => {
        const err = new ServiceCycleError('cycle', ['api', 'http']);
        expect(err).toBeInstanceOf(Error);
        expect(err.cycle).toEqual(['api', 'http']);
    });
});
