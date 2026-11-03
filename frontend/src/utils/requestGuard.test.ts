import { describe, expect, it, vi } from 'vitest';
import { RequestGuard } from './requestGuard';

describe('RequestGuard', () => {
  it('accepts only the most recently started request', () => {
    const guard = new RequestGuard();
    const first = guard.begin();
    const second = guard.begin();

    expect(guard.isCurrent(first)).toBe(false);
    expect(guard.isCurrent(second)).toBe(true);
  });

  it('invalidates outstanding requests when a view is replaced', () => {
    const guard = new RequestGuard();
    const request = guard.begin();

    guard.invalidate();

    expect(guard.isCurrent(request)).toBe(false);
  });

  it('keeps entity C current when entity A and B requests finish out of order', async () => {
    const guard = new RequestGuard();
    const applied: string[] = [];
    const deferred = () => {
      let resolve!: (value: string) => void;
      const promise = new Promise<string>((done) => { resolve = done; });
      return { promise, resolve };
    };
    const a = deferred();
    const b = deferred();
    const c = deferred();
    const requests = [
      { token: guard.begin(), entityId: 'A', response: a.promise },
      { token: guard.begin(), entityId: 'B', response: b.promise },
      { token: guard.begin(), entityId: 'C', response: c.promise },
    ];
    const applyIfCurrent = async (request: (typeof requests)[number]) => {
      const value = await request.response;
      if (guard.isCurrent(request.token)) applied.push(`${request.entityId}:${value}`);
    };
    const completions = requests.map(applyIfCurrent);

    c.resolve('metadata-C');
    await Promise.resolve();
    b.resolve('metadata-B');
    await Promise.resolve();
    a.resolve('metadata-A');
    await Promise.all(completions);

    expect(applied).toEqual(['C:metadata-C']);
  });

  it('ignores a stale rejection after the current entity request succeeds', async () => {
    const guard = new RequestGuard();
    let currentError: string | null = null;
    const staleToken = guard.begin();
    const activeToken = guard.begin();
    let rejectStale!: (error: Error) => void;
    const staleRequest = new Promise<never>((_resolve, reject) => { rejectStale = reject; });
    const activeRequest = Promise.resolve('entity B loaded');

    const activeValue = await activeRequest;
    if (guard.isCurrent(activeToken)) expect(activeValue).toBe('entity B loaded');
    rejectStale(new Error('entity A failed'));
    try {
      await staleRequest;
    } catch (error) {
      if (guard.isCurrent(staleToken)) currentError = (error as Error).message;
    }

    expect(currentError).toBeNull();
  });

  it('ignores a save result after the selected entity view has been invalidated', () => {
    const guard = new RequestGuard();
    const entityASave = guard.begin();
    const toast = vi.fn();

    // Entity A's detail view unmounts after selecting B. The old request may still
    // reach the API, but its completion no longer updates view state or feedback.
    guard.invalidate();
    if (guard.isCurrent(entityASave)) toast('A saved');

    expect(toast).not.toHaveBeenCalled();
  });

  it('resets ownership when a detail closes and the same entity is reopened', () => {
    const guard = new RequestGuard();
    const firstOpen = guard.begin();
    guard.invalidate();
    const reopened = guard.begin();

    expect(guard.isCurrent(firstOpen)).toBe(false);
    expect(guard.isCurrent(reopened)).toBe(true);
  });

  it('invalidates definitions and values when the same entity changes type', () => {
    const guard = new RequestGuard();
    const vehicleTypeRequest = guard.begin();
    const facilityTypeRequest = guard.begin();

    expect(guard.isCurrent(vehicleTypeRequest)).toBe(false);
    expect(guard.isCurrent(facilityTypeRequest)).toBe(true);
  });
});
