export type PrimaryOverlay = 'attribute-definitions' | 'data-operations' | 'geofences';
export type AppView = 'map' | 'monitoring';

export interface AppNavigationState {
  view: AppView;
  overlay: PrimaryOverlay | null;
}

export type AppNavigationAction =
  | { type: 'open-overlay'; overlay: PrimaryOverlay }
  | { type: 'close-overlay' }
  | { type: 'toggle-monitoring' };

export const initialAppNavigationState: AppNavigationState = {
  view: 'map',
  overlay: null,
};

export function appNavigationReducer(
  state: AppNavigationState,
  action: AppNavigationAction
): AppNavigationState {
  switch (action.type) {
    case 'open-overlay':
      return { view: 'map', overlay: action.overlay };
    case 'close-overlay':
      return { ...state, overlay: null };
    case 'toggle-monitoring':
      return state.view === 'monitoring'
        ? { view: 'map', overlay: null }
        : { view: 'monitoring', overlay: null };
  }
}
