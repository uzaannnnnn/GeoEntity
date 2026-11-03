export type FrameScheduler = (callback: () => void) => number;
export type FrameCanceller = (handle: number) => void;

export function createCoalescedFrameTask(
  task: () => void,
  schedule: FrameScheduler = (callback) => requestAnimationFrame(callback),
  cancel: FrameCanceller = (handle) => cancelAnimationFrame(handle)
) {
  let pendingFrame: number | null = null;

  return {
    request() {
      if (pendingFrame !== null) return;
      pendingFrame = schedule(() => {
        pendingFrame = null;
        task();
      });
    },
    cancel() {
      if (pendingFrame === null) return;
      cancel(pendingFrame);
      pendingFrame = null;
    },
  };
}
