import { describe, expect, it, vi } from 'vitest';
import { createCoalescedFrameTask } from './coalescedFrameTask';

describe('createCoalescedFrameTask', () => {
  it('coalesces requests into one frame', () => {
    const task = vi.fn();
    const callbacks: Array<() => void> = [];
    const schedule = vi.fn((callback: () => void) => {
      callbacks.push(callback);
      return callbacks.length;
    });
    const queued = createCoalescedFrameTask(task, schedule);

    queued.request();
    queued.request();
    queued.request();

    expect(schedule).toHaveBeenCalledTimes(1);
    callbacks[0]();
    expect(task).toHaveBeenCalledTimes(1);
  });

  it('cancels a pending frame and allows a later request', () => {
    const task = vi.fn();
    const callbacks: Array<() => void> = [];
    const cancel = vi.fn();
    const queued = createCoalescedFrameTask(
      task,
      (callback) => {
        callbacks.push(callback);
        return callbacks.length;
      },
      cancel
    );

    queued.request();
    queued.cancel();
    expect(cancel).toHaveBeenCalledWith(1);
    queued.request();
    callbacks[1]();

    expect(task).toHaveBeenCalledTimes(1);
  });
});
