import { useState, useEffect, useCallback, useMemo, useRef, FormEvent } from 'react';
import {
  Entity,
  EntityInput,
  MapEntity,
  MapBoundsParams,
  EntityPageMeta,
  SidePanelMode,
} from './types/entity';
import { entitiesApi } from './api/entities';
import { ApiError } from './api/client';
import { Header } from './components/Header';
import { MapView } from './components/MapView';
import { EntityList, EntityListFilters } from './components/EntityList';
import { EntityDetail } from './components/EntityDetail';
import { EntityForm } from './components/EntityForm';
import { DeleteModal } from './components/DeleteModal';
import { ToastContainer, ToastMessage } from './components/Toast';
import { AttributeDefinitionsModal } from './components/advanced/AttributeDefinitionsModal';
import { DataExchangeModal } from './components/exchange/DataExchangeModal';
import { ImportResult } from './types/dataExchange';
import { removeEntity, upsertEntity } from './utils/entityState';
import { RequestGuard } from './utils/requestGuard';
import { calculateExtentFitTarget } from './utils/mapViewport';
import { FitTarget } from './utils/mapBounds';
import { AnyMarkerEntity } from './components/map/EntityMarkerLayer';
import { geofencesApi } from './api/geofences';
import { GeofenceDefinition } from './types/geofence';
import { GeofenceManagerModal, GeofenceModalTab } from './components/geofences/GeofenceManagerModal';
import { authApi } from './api/auth';

function AuthenticatedApp() {
  // 1. Paginated Entity List Data Source
  const [entities, setEntities] = useState<Entity[]>([]);
  const [entityMeta, setEntityMeta] = useState<EntityPageMeta | null>(null);
  const [isLoadingList, setIsLoadingList] = useState<boolean>(true);
  const [isLoadingMore, setIsLoadingMore] = useState<boolean>(false);
  const [listError, setListError] = useState<string | null>(null);

  // Active Search & Category Filters (Shared across List and Map)
  const [activeFilters, setActiveFilters] = useState<EntityListFilters>({
    q: '',
    type: 'all',
    status: 'all',
  });

  // 2. Viewport-Bounded Map Marker Data Source
  const [mapMarkers, setMapMarkers] = useState<MapEntity[]>([]);
  const [currentViewportBounds, setCurrentViewportBounds] = useState<MapBoundsParams | null>(null);
  const [isMarkersLoading, setIsMarkersLoading] = useState<boolean>(false);
  const [mapLimitExceeded, setMapLimitExceeded] = useState<boolean>(false);
  const [mapError, setMapError] = useState<string | null>(null);
  const [fitTargetRequest, setFitTargetRequest] = useState<{
    target: FitTarget;
    timestamp: number;
  } | null>(null);

  // Selected Entity and Panel State
  const [selectedEntity, setSelectedEntity] = useState<Entity | null>(null);
  const [sidePanelMode, setSidePanelMode] = useState<SidePanelMode>('list');
  const [isSelectingLocation, setIsSelectingLocation] = useState<boolean>(false);
  const [draftLocation, setDraftLocation] = useState<{ lat: number; lng: number } | null>(null);
  const [centerRequest, setCenterRequest] = useState(0);
  const [isDefinitionsModalOpen, setIsDefinitionsModalOpen] = useState<boolean>(false);
  const [isDataExchangeModalOpen, setIsDataExchangeModalOpen] = useState<boolean>(false);
  const [isGeofencesModalOpen, setIsGeofencesModalOpen] = useState<boolean>(false);
  const [geofencesModalInitialTab, setGeofencesModalInitialTab] = useState<GeofenceModalTab>('list');
  const [geofences, setGeofences] = useState<GeofenceDefinition[]>([]);
  const [selectedGeofence, setSelectedGeofence] = useState<GeofenceDefinition | null>(null);
  const [isHealthOk, setIsHealthOk] = useState<boolean | null>(null);

  // Request race condition guards
  const listRequestGuard = useRef(new RequestGuard()).current;
  const mapRequestGuard = useRef(new RequestGuard()).current;
  const panelRequestGuard = useRef(new RequestGuard()).current;
  const mapDebounceTimerRef = useRef<NodeJS.Timeout | null>(null);

  // Form error feedback
  const [formBackendError, setFormBackendError] = useState<string | null>(null);
  const [formBackendField, setFormBackendField] = useState<string | undefined>(undefined);

  // Deletion modal state
  const [entityToDelete, setEntityToDelete] = useState<Entity | null>(null);
  const [isDeleting, setIsDeleting] = useState<boolean>(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  // Toast notifications
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  const addToast = useCallback(
    (type: 'success' | 'error' | 'info', message: string, title?: string) => {
      const newToast: ToastMessage = {
        id: `${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
        type,
        title,
        message,
      };
      setToasts((prev) => [...prev, newToast]);
    },
    []
  );

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  // Fetch paginated entity list from /api/entities/query
  const fetchEntitiesList = useCallback(
    async (
      filters: EntityListFilters,
      cursor?: string,
      isLoadMore = false
    ) => {
      const request = listRequestGuard.begin();
      if (isLoadMore) {
        setIsLoadingMore(true);
      } else {
        setIsLoadingList(true);
      }
      setListError(null);

      try {
        const page = await entitiesApi.query({
          q: filters.q,
          type: filters.type,
          status: filters.status,
          limit: 50,
          cursor,
        });

        if (!listRequestGuard.isCurrent(request)) return;

        if (isLoadMore) {
          setEntities((prev) => {
            const existingIds = new Set(prev.map((e) => e.id));
            const fresh = page.data.filter((e) => !existingIds.has(e.id));
            return [...prev, ...fresh];
          });
        } else {
          setEntities(page.data);
        }
        setEntityMeta(page.meta);
      } catch (err) {
        if (!listRequestGuard.isCurrent(request)) return;
        const message =
          err instanceof ApiError
            ? err.message
            : err instanceof Error
            ? err.message
            : 'Failed to communicate with GeoEntity API';
        setListError(message);
        addToast('error', message, 'Connection Error');
      } finally {
        if (listRequestGuard.isCurrent(request)) {
          setIsLoadingList(false);
          setIsLoadingMore(false);
        }
      }
    },
    [addToast, listRequestGuard]
  );

  // Fetch viewport-bounded markers from /api/entities/map
  const fetchMapMarkers = useCallback(
    async (bounds: MapBoundsParams, filters: EntityListFilters) => {
      const request = mapRequestGuard.begin();
      setIsMarkersLoading(true);
      setMapError(null);

      try {
        const markers = await entitiesApi.getMapEntities({
          ...bounds,
          q: filters.q,
          type: filters.type,
          status: filters.status,
        });

        if (!mapRequestGuard.isCurrent(request)) return;
        setMapMarkers(markers);
        setMapLimitExceeded(false);
      } catch (err) {
        if (!mapRequestGuard.isCurrent(request)) return;
        if (err instanceof ApiError && (err.code === 'MAP_RESULT_LIMIT' || err.status === 413)) {
          setMapLimitExceeded(true);
        } else {
          const msg = err instanceof Error ? err.message : 'Failed to fetch map markers';
          setMapError(msg);
        }
      } finally {
        if (mapRequestGuard.isCurrent(request)) {
          setIsMarkersLoading(false);
        }
      }
    },
    [mapRequestGuard]
  );

  // Check backend health
  const checkHealth = useCallback(async () => {
    try {
      const res = await entitiesApi.getHealth();
      setIsHealthOk(res?.status === 'ok');
    } catch {
      setIsHealthOk(false);
    }
  }, []);

  // Fetch geofences list
  const fetchGeofences = useCallback(async () => {
    try {
      const res = await geofencesApi.list({ limit: 100 });
      setGeofences(res.data);
    } catch (err) {
      console.error('Failed to load geofences', err);
    }
  }, []);

  // Initial load
  useEffect(() => {
    fetchEntitiesList(activeFilters);
    fetchGeofences();
    checkHealth();
  }, [fetchEntitiesList, fetchGeofences, checkHealth]);

  // Debounced map markers fetch when viewport bounds or active filters change
  useEffect(() => {
    if (!currentViewportBounds) return;

    if (mapDebounceTimerRef.current) {
      clearTimeout(mapDebounceTimerRef.current);
    }

    mapDebounceTimerRef.current = setTimeout(() => {
      fetchMapMarkers(currentViewportBounds, activeFilters);
    }, 250);

    return () => {
      if (mapDebounceTimerRef.current) {
        clearTimeout(mapDebounceTimerRef.current);
      }
    };
  }, [currentViewportBounds, activeFilters, fetchMapMarkers]);

  // Filter change handler: resets list pagination and updates filters
  const handleFilterChange = useCallback(
    (newFilters: EntityListFilters) => {
      setActiveFilters(newFilters);
      listRequestGuard.invalidate();
      mapRequestGuard.invalidate();
      fetchEntitiesList(newFilters, undefined, false);
    },
    [fetchEntitiesList, listRequestGuard, mapRequestGuard]
  );

  // Load more pages
  const handleLoadMore = useCallback(() => {
    if (!entityMeta?.next_cursor || isLoadingMore) return;
    fetchEntitiesList(activeFilters, entityMeta.next_cursor, true);
  }, [activeFilters, entityMeta?.next_cursor, fetchEntitiesList, isLoadingMore]);

  // Fit All Entities using server extent endpoint
  const handleFitAll = useCallback(async () => {
    try {
      const extentData = await entitiesApi.getMapExtent({
        q: activeFilters.q,
        type: activeFilters.type,
        status: activeFilters.status,
      });

      if (!extentData || extentData.count === 0 || !extentData.bounds) {
        addToast('info', 'No matching entities to fit.', 'Fit All');
        return;
      }

      const target = calculateExtentFitTarget(extentData);
      setFitTargetRequest({ target, timestamp: Date.now() });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to calculate entity extent';
      addToast('error', msg, 'Fit All Failed');
    }
  }, [activeFilters, addToast]);

  // Handle entity selection from list
  const handleSelectEntity = useCallback(
    (entity: Entity) => {
      panelRequestGuard.invalidate();
      setSelectedEntity(entity);
      setSidePanelMode('detail');
      setIsSelectingLocation(false);
      setDraftLocation(null);
    },
    [panelRequestGuard]
  );

  // Handle entity selection from map marker (may be MapEntity or full Entity)
  const handleSelectMapMarker = useCallback(
    async (marker: AnyMarkerEntity) => {
      panelRequestGuard.invalidate();
      setIsSelectingLocation(false);
      setDraftLocation(null);

      // Check if entity is already in current list
      const existing = entities.find((e) => e.id === marker.id);
      if (existing) {
        setSelectedEntity(existing);
        setSidePanelMode('detail');
        return;
      }

      // Initial optimistic entity for instant panel responsiveness
      const initial: Entity = {
        id: marker.id,
        name: marker.name,
        type: marker.type,
        status: marker.status,
        latitude: marker.latitude,
        longitude: marker.longitude,
        description: 'description' in marker && marker.description ? marker.description : '',
        created_at: 'created_at' in marker ? (marker as Entity).created_at : '',
        updated_at: 'updated_at' in marker ? (marker as Entity).updated_at : '',
      };
      setSelectedEntity(initial);
      setSidePanelMode('detail');

      // Fetch authoritative full entity details
      try {
        const full = await entitiesApi.getById(marker.id);
        setSelectedEntity(full);
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to load entity details';
        addToast('error', msg, 'Error');
      }
    },
    [entities, panelRequestGuard, addToast]
  );

  const handleBackToList = useCallback(() => {
    panelRequestGuard.invalidate();
    setSidePanelMode('list');
    setSelectedEntity(null);
    setIsSelectingLocation(false);
    setDraftLocation(null);
  }, [panelRequestGuard]);

  // Handlers for Add / Create
  const handleStartCreate = useCallback(() => {
    panelRequestGuard.invalidate();
    setSelectedEntity(null);
    setSidePanelMode('create');
    setIsSelectingLocation(true);
    setDraftLocation(null);
    setFormBackendError(null);
    setFormBackendField(undefined);
  }, [panelRequestGuard]);

  // Handlers for Edit
  const handleStartEdit = useCallback(
    (entity: AnyMarkerEntity) => {
      panelRequestGuard.invalidate();
      const existing = entities.find((e) => e.id === entity.id) || (entity as Entity);
      setSelectedEntity(existing);
      setSidePanelMode('edit');
      setIsSelectingLocation(false);
      setDraftLocation({ lat: entity.latitude, lng: entity.longitude });
      setFormBackendError(null);
      setFormBackendField(undefined);
    },
    [entities, panelRequestGuard]
  );

  const handleCenterMap = useCallback(
    (entity: Entity) => {
      panelRequestGuard.invalidate();
      setSelectedEntity(entity);
      setCenterRequest((request) => request + 1);
    },
    [panelRequestGuard]
  );

  // Location picking on map
  const handleLocationPicked = useCallback(
    (lat: number, lng: number) => {
      setDraftLocation({ lat, lng });
      addToast('info', `Picked: ${lat.toFixed(5)}, ${lng.toFixed(5)}`, 'Location Selected');
    },
    [addToast]
  );

  const handleDraftDragEnd = useCallback((lat: number, lng: number) => {
    setDraftLocation({ lat, lng });
  }, []);

  // Form submissions
  const handleCreateSubmit = useCallback(
    async (payload: EntityInput) => {
      const request = panelRequestGuard.begin();
      setFormBackendError(null);
      setFormBackendField(undefined);

      try {
        const created = await entitiesApi.create(payload);
        addToast('success', `Entity "${created.name}" created successfully.`, 'Created');

        // Upsert into list and map
        setEntities((current) => upsertEntity(current, created));
        setMapMarkers((current) => [created, ...current.filter((m) => m.id !== created.id)]);
        setEntityMeta((prev) =>
          prev
            ? {
                ...prev,
                matching_count: prev.matching_count + 1,
                status_counts: {
                  ...prev.status_counts,
                  [created.status]: (prev.status_counts[created.status] || 0) + 1,
                },
              }
            : null
        );

        if (panelRequestGuard.isCurrent(request)) {
          setSelectedEntity(created);
          setSidePanelMode('detail');
          setIsSelectingLocation(false);
          setDraftLocation(null);
        }
        fetchGeofences();
      } catch (err) {
        if (!panelRequestGuard.isCurrent(request)) return;
        if (err instanceof ApiError) {
          setFormBackendError(err.message);
          setFormBackendField(err.field);
          addToast('error', err.message, 'Create Failed');
        } else {
          const msg = err instanceof Error ? err.message : 'Unknown error during entity creation';
          setFormBackendError(msg);
          addToast('error', msg, 'Create Failed');
        }
        throw err;
      }
    },
    [addToast, panelRequestGuard, fetchGeofences]
  );

  const handleUpdateSubmit = useCallback(
    async (payload: EntityInput) => {
      if (!selectedEntity) return;
      const request = panelRequestGuard.begin();
      const updatedEntityId = selectedEntity.id;

      setFormBackendError(null);
      setFormBackendField(undefined);

      try {
        const updated = await entitiesApi.update(selectedEntity.id, payload);
        addToast('success', `Entity "${updated.name}" updated successfully.`, 'Updated');

        setEntities((current) => upsertEntity(current, updated));
        setMapMarkers((current) =>
          current.map((m) =>
            m.id === updated.id
              ? {
                  id: updated.id,
                  name: updated.name,
                  type: updated.type,
                  status: updated.status,
                  latitude: updated.latitude,
                  longitude: updated.longitude,
                }
              : m
          )
        );

        if (panelRequestGuard.isCurrent(request)) {
          setSelectedEntity((current) => (current?.id === updatedEntityId ? updated : current));
          setSidePanelMode((current) => (current === 'edit' ? 'detail' : current));
          setIsSelectingLocation(false);
          setDraftLocation(null);
        }
        fetchGeofences();
      } catch (err) {
        if (!panelRequestGuard.isCurrent(request)) return;
        if (err instanceof ApiError) {
          setFormBackendError(err.message);
          setFormBackendField(err.field);
          addToast('error', err.message, 'Update Failed');
        } else {
          const msg = err instanceof Error ? err.message : 'Unknown error during entity update';
          setFormBackendError(msg);
          addToast('error', msg, 'Update Failed');
        }
        throw err;
      }
    },
    [selectedEntity, addToast, panelRequestGuard, fetchGeofences]
  );

  // Deletion handling
  const handleStartDelete = useCallback((entity: Entity) => {
    setEntityToDelete(entity);
    setDeleteError(null);
  }, []);

  const handleConfirmDelete = useCallback(async () => {
    if (!entityToDelete) return;
    setIsDeleting(true);
    setDeleteError(null);

    try {
      await entitiesApi.delete(entityToDelete.id);
      addToast('success', `Entity "${entityToDelete.name}" deleted successfully.`, 'Deleted');

      setEntities((current) => removeEntity(current, entityToDelete.id));
      setMapMarkers((current) => current.filter((m) => m.id !== entityToDelete.id));
      setEntityMeta((prev) =>
        prev
          ? {
              ...prev,
              matching_count: Math.max(0, prev.matching_count - 1),
              status_counts: {
                ...prev.status_counts,
                [entityToDelete.status]: Math.max(
                  0,
                  (prev.status_counts[entityToDelete.status] || 1) - 1
                ),
              },
            }
          : null
      );

      setEntityToDelete(null);
      if (selectedEntity?.id === entityToDelete.id) {
        setSelectedEntity(null);
        setSidePanelMode('list');
      }
      fetchGeofences();
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to delete entity';
      setDeleteError(msg);
      addToast('error', msg, 'Delete Failed');
    } finally {
      setIsDeleting(false);
    }
  }, [entityToDelete, selectedEntity, addToast, fetchGeofences]);

  // Header status counts: derived authoritatively from server meta
  const { totalCount, activeCount, maintenanceCount, inactiveCount } = useMemo(() => {
    if (entityMeta) {
      return {
        totalCount: entityMeta.matching_count,
        activeCount: entityMeta.status_counts?.active ?? 0,
        maintenanceCount: entityMeta.status_counts?.maintenance ?? 0,
        inactiveCount: entityMeta.status_counts?.inactive ?? 0,
      };
    }
    return {
      totalCount: entities.length,
      activeCount: entities.filter((e) => e.status === 'active').length,
      maintenanceCount: entities.filter((e) => e.status === 'maintenance').length,
      inactiveCount: entities.filter((e) => e.status === 'inactive').length,
    };
  }, [entityMeta, entities]);

  const hasMore = Boolean(entityMeta?.next_cursor && entityMeta.next_cursor.trim() !== '');

  const handleImportSuccess = useCallback(
    (result: ImportResult) => {
      addToast(
        'success',
        `Successfully imported ${result.created} ${
          result.created === 1 ? 'entity' : 'entities'
        } via ${result.format.toUpperCase()}!`,
        'Import Completed'
      );
      fetchEntitiesList(activeFilters);
      fetchGeofences();
      if (currentViewportBounds) {
        fetchMapMarkers(currentViewportBounds, activeFilters);
      }
      handleFitAll();
      checkHealth();
    },
    [addToast, fetchEntitiesList, fetchGeofences, activeFilters, currentViewportBounds, fetchMapMarkers, handleFitAll, checkHealth]
  );

  return (
    <div className="geoentity-app">
      <Header
        totalCount={totalCount}
        activeCount={activeCount}
        maintenanceCount={maintenanceCount}
        inactiveCount={inactiveCount}
        isHealthOk={isHealthOk}
        isLoading={isLoadingList}
        onAddClick={handleStartCreate}
        onRefreshClick={() => {
          fetchEntitiesList(activeFilters);
          fetchGeofences();
          if (currentViewportBounds) {
            fetchMapMarkers(currentViewportBounds, activeFilters);
          }
          checkHealth();
        }}
        onManageDefinitionsClick={() => setIsDefinitionsModalOpen(true)}
        onDataExchangeClick={() => setIsDataExchangeModalOpen(true)}
        onGeofencesClick={() => {
          setGeofencesModalInitialTab('list');
          setIsGeofencesModalOpen(true);
        }}
        isGeofencesActive={isGeofencesModalOpen}
        geofenceCount={geofences.length}
        isCreateActive={sidePanelMode === 'create'}
      />

      <main className="app-main-layout">
        {/* Interactive Leaflet Map is the primary working area */}
        <section className="map-section" aria-label="Geographic Entity Map">
          <MapView
            mapMarkers={mapMarkers}
            selectedEntity={selectedEntity}
            centerRequest={centerRequest}
            draftLocation={draftLocation}
            isSelectingLocation={isSelectingLocation}
            onSelectEntity={handleSelectMapMarker}
            onLocationSelected={handleLocationPicked}
            onDraftDragEnd={handleDraftDragEnd}
            onBoundsChange={setCurrentViewportBounds}
            onFitAllClick={handleFitAll}
            matchingCount={totalCount}
            isMarkersLoading={isMarkersLoading}
            mapLimitExceeded={mapLimitExceeded}
            mapError={mapError}
            fitTargetRequest={fitTargetRequest}
            onViewDetailsClick={handleSelectMapMarker}
            onEditClick={handleStartEdit}
            geofences={geofences}
            selectedGeofenceId={selectedGeofence?.id}
            onSelectGeofence={(gf) => setSelectedGeofence(gf)}
            onViewGeofenceEvents={(gf) => {
              setSelectedGeofence(gf);
              setGeofencesModalInitialTab('events');
              setIsGeofencesModalOpen(true);
            }}
          />
        </section>

        {/* Side Panel for list, details, and forms */}
        <aside className="side-panel-section" aria-label="Entity Management Panel">
          {sidePanelMode === 'list' && (
            <EntityList
              entities={entities}
              selectedEntity={selectedEntity}
              isLoading={isLoadingList}
              isLoadingMore={isLoadingMore}
              error={listError}
              matchingCount={totalCount}
              hasMore={hasMore}
              activeFilters={activeFilters}
              onFilterChange={handleFilterChange}
              onLoadMore={handleLoadMore}
              onSelectEntity={handleSelectEntity}
              onAddNewClick={handleStartCreate}
              onRetry={() => {
                fetchEntitiesList(activeFilters);
                checkHealth();
              }}
            />
          )}

          {sidePanelMode === 'detail' && selectedEntity && (
            <EntityDetail
              key={selectedEntity.id}
              entity={selectedEntity}
              onBack={handleBackToList}
              onEdit={handleStartEdit}
              onDelete={handleStartDelete}
              onCenterMap={handleCenterMap}
              onToast={addToast}
              onViewGeofenceEvents={(geofenceId) => {
                const gf = geofences.find((g) => g.id === geofenceId);
                if (gf) {
                  setSelectedGeofence(gf);
                }
                setGeofencesModalInitialTab('events');
                setIsGeofencesModalOpen(true);
              }}
              onSelectGeofenceOnMap={(geofenceId) => {
                const gf = geofences.find((g) => g.id === geofenceId);
                if (gf) {
                  setSelectedGeofence(gf);
                }
              }}
            />
          )}

          {sidePanelMode === 'create' && (
            <EntityForm
              mode="create"
              draftLocation={draftLocation}
              isSelectingLocation={isSelectingLocation}
              onToggleSelectLocation={() => setIsSelectingLocation((prev) => !prev)}
              onSubmit={handleCreateSubmit}
              onCancel={handleBackToList}
              backendError={formBackendError}
              backendErrorField={formBackendField}
            />
          )}

          {sidePanelMode === 'edit' && selectedEntity && (
            <EntityForm
              mode="edit"
              initialEntity={selectedEntity}
              draftLocation={draftLocation}
              isSelectingLocation={isSelectingLocation}
              onToggleSelectLocation={() => setIsSelectingLocation((prev) => !prev)}
              onSubmit={handleUpdateSubmit}
              onCancel={() => setSidePanelMode('detail')}
              backendError={formBackendError}
              backendErrorField={formBackendField}
            />
          )}
        </aside>
      </main>

      {/* Delete confirmation dialog */}
      <DeleteModal
        entity={entityToDelete}
        isOpen={Boolean(entityToDelete)}
        isDeleting={isDeleting}
        error={deleteError}
        onConfirm={handleConfirmDelete}
        onCancel={() => {
          if (!isDeleting) {
            setEntityToDelete(null);
            setDeleteError(null);
          }
        }}
      />

      {/* Attribute Definitions Schema Manager Modal */}
      <AttributeDefinitionsModal
        isOpen={isDefinitionsModalOpen}
        onClose={() => setIsDefinitionsModalOpen(false)}
        onToast={addToast}
      />

      {/* Data Operations (CSV / GeoJSON Import & Export) Modal */}
      <DataExchangeModal
        isOpen={isDataExchangeModalOpen}
        onClose={() => setIsDataExchangeModalOpen(false)}
        activeFilters={activeFilters}
        matchingCount={totalCount}
        onImportSuccess={handleImportSuccess}
      />

      {/* Geofence Management and Transition Events Modal */}
      <GeofenceManagerModal
        isOpen={isGeofencesModalOpen}
        onClose={() => setIsGeofencesModalOpen(false)}
        geofences={geofences}
        selectedGeofence={selectedGeofence}
        onSelectGeofence={setSelectedGeofence}
        onRefreshGeofences={fetchGeofences}
        mapCenter={{
          lat: selectedEntity?.latitude ?? draftLocation?.lat ?? -6.2088,
          lng: selectedEntity?.longitude ?? draftLocation?.lng ?? 106.8456,
        }}
        onSelectEntity={(entityId) => {
          const found = entities.find((e) => e.id === entityId);
          if (found) {
            handleSelectEntity(found);
          } else {
            handleSelectMapMarker({
              id: entityId,
              name: '',
              type: 'facility',
              status: 'active',
              latitude: 0,
              longitude: 0,
            });
          }
          setIsGeofencesModalOpen(false);
        }}
        onToast={addToast}
        initialTab={geofencesModalInitialTab}
      />

      {/* Notification Toast Stack */}
      <ToastContainer toasts={toasts} onDismiss={removeToast} />
    </div>
  );
}

export function App() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [signOutError, setSignOutError] = useState('');

  useEffect(() => {
    let mounted = true;
    authApi.session().then((state) => { if (mounted) setAuthenticated(state.authenticated); }).catch(() => { if (mounted) setAuthenticated(false); });
    const onExpired = () => { setAuthenticated(false); setError('Your session expired. Sign in again.'); };
    window.addEventListener('geoentity:auth-required', onExpired);
    return () => { mounted = false; window.removeEventListener('geoentity:auth-required', onExpired); };
  }, []);

  async function signIn(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('');
    try { await authApi.login(password); setPassword(''); setAuthenticated(true); }
    catch { setError('Sign in failed. Check the passphrase and try again.'); }
    finally { setBusy(false); }
  }

  async function signOut() {
    setSignOutError('');
    try { await authApi.logout(); setAuthenticated(false); }
    catch { setSignOutError('Sign out failed. Please try again.'); }
  }

  if (authenticated === null) return <main className="auth-screen"><p>Checking session…</p></main>;
  if (!authenticated) return (
    <main className="auth-screen">
      <form className="auth-card" onSubmit={signIn}>
        <h1>GeoEntity</h1><p>Sign in to manage your data.</p>
        <label htmlFor="admin-passphrase">Administrator passphrase</label>
        <input id="admin-passphrase" type="password" autoComplete="current-password" required value={password} onChange={(event) => setPassword(event.target.value)} />
        {error && <p className="auth-error" role="alert">{error}</p>}
        <button type="submit" disabled={busy || password.length === 0}>{busy ? 'Signing in…' : 'Sign in'}</button>
      </form>
    </main>
  );
  return <><div className="auth-session-controls">{signOutError && <span role="alert">{signOutError}</span>}<button className="auth-signout" onClick={signOut}>Sign out</button></div><AuthenticatedApp /></>;
}
