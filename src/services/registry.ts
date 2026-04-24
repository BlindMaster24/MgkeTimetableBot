import type { AppServiceName } from '../app';

export type ServiceDependencySpec = {
    required: readonly AppServiceName[];
    optional: readonly AppServiceName[];
};

export const serviceDependencies: Record<AppServiceName, ServiceDependencySpec> = {
    http: { required: [], optional: [] },
    parser: { required: [], optional: [] },
    timetable: { required: [], optional: ['parser'] },
    image: { required: [], optional: ['http'] },
    bot: { required: ['parser'], optional: [] },
    tg: { required: ['bot'], optional: ['image', 'timetable'] },
    vk: { required: ['bot'], optional: ['image', 'timetable'] },
    viber: { required: ['bot', 'http'], optional: ['image', 'timetable'] },
    api: { required: ['http'], optional: ['timetable'] },
    alice: { required: ['http'], optional: ['timetable'] },
    vkApp: { required: ['http'], optional: [] },
    google_calendar: { required: ['http', 'bot', 'parser', 'timetable'], optional: [] }
};

export class ServiceDependencyError extends Error {
    constructor(
        message: string,
        public readonly missing: Array<{ service: AppServiceName; requires: AppServiceName }>
    ) {
        super(message);
        this.name = 'ServiceDependencyError';
    }
}

export class ServiceCycleError extends Error {
    constructor(
        message: string,
        public readonly cycle: AppServiceName[]
    ) {
        super(message);
        this.name = 'ServiceCycleError';
    }
}

export function validateServiceDependencies(enabled: readonly AppServiceName[]): void {
    const enabledSet = new Set<AppServiceName>(enabled);
    const missing: Array<{ service: AppServiceName; requires: AppServiceName }> = [];

    for (const service of enabledSet) {
        const spec = serviceDependencies[service];
        for (const dep of spec.required) {
            if (!enabledSet.has(dep)) {
                missing.push({ service, requires: dep });
            }
        }
    }

    if (missing.length > 0) {
        const lines = missing.map(({ service, requires }) => `  - '${service}' requires '${requires}' to be enabled`);
        throw new ServiceDependencyError(
            `Service dependency check failed:\n${lines.join('\n')}\n` +
                `Add the missing services to config.services or remove the dependents.`,
            missing
        );
    }
}

export function topologicalSort(enabled: readonly AppServiceName[]): AppServiceName[] {
    const enabledSet = new Set<AppServiceName>(enabled);
    const indegree = new Map<AppServiceName, number>();
    const edges = new Map<AppServiceName, Set<AppServiceName>>();

    for (const service of enabledSet) {
        indegree.set(service, 0);
        edges.set(service, new Set());
    }

    for (const service of enabledSet) {
        const spec = serviceDependencies[service];
        const deps = [...spec.required, ...spec.optional];
        for (const dep of deps) {
            if (!enabledSet.has(dep)) continue;
            const successors = edges.get(dep)!;
            if (successors.has(service)) continue;
            successors.add(service);
            indegree.set(service, (indegree.get(service) ?? 0) + 1);
        }
    }

    const order: AppServiceName[] = [];
    const queue: AppServiceName[] = [];
    const inputIndex = new Map<AppServiceName, number>();
    enabled.forEach((service, index) => inputIndex.set(service, index));

    const byInputOrder = (a: AppServiceName, b: AppServiceName) => (inputIndex.get(a) ?? 0) - (inputIndex.get(b) ?? 0);

    for (const service of enabledSet) {
        if (indegree.get(service) === 0) queue.push(service);
    }
    queue.sort(byInputOrder);

    while (queue.length > 0) {
        const service = queue.shift()!;
        order.push(service);
        for (const next of edges.get(service) ?? []) {
            const deg = (indegree.get(next) ?? 0) - 1;
            indegree.set(next, deg);
            if (deg === 0) {
                queue.push(next);
                queue.sort(byInputOrder);
            }
        }
    }

    if (order.length !== enabledSet.size) {
        const remaining = [...enabledSet].filter((s) => !order.includes(s));
        throw new ServiceCycleError(
            `Service dependency graph contains a cycle involving: ${remaining.join(', ')}`,
            remaining
        );
    }

    return order;
}
