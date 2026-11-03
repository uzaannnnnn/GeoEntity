import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { Entity, EntityInput, SidePanelMode } from './types/entity';
import { entitiesApi } from './api/entities';
import { ApiError } from './api/client';
import { Header } from './components/Header';
import { MapView } from './components/MapView';
import { EntityList } from './components/EntityList';
import { EntityDetail } from './components/EntityDetail';
import { EntityForm } from './components/EntityForm';
import { DeleteModal } from './components/DeleteModal';
import { ToastContainer, ToastMessage } from './components/Toast';
import { AttributeDefinitionsModal } from './components/advanced/AttributeDefinitionsModal';
import { removeEntity, upsertEntity } from './utils/entityState';
import { RequestGuard } from './utils/requestGuard';

export function App() {
  // Core entity state loaded strictly from backend
  const [entities, setEntities] = useState<Entity[]>([]);
  const [selectedEntity, setSelectedEntity] = useState<Entity | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [listError, setListError] = useState<string | null>(null);
  const [isHealthOk, setIsHealthOk] = useState<boolean | null>(null);

  // UI / Panel modes
  const [sidePanelMode, setSidePanelMode] = useState<SidePanelMode>('list');
  const [isSelectingLocation, setIsSelectingLocation] = useState<boolean>(false);
  const [draftLocation, setDraftLocation] = useState<{ lat: number; lng: number } | null>(null);
  const [shouldFitBounds, setShouldFitBounds] = useState<boolean>(false);
  const [centerRequest, setCenterRequest] = useState(0);
  const [isDefinitionsModalOpen, setIsDefinitionsModalOpen] = useState<boolean>(false);
  const panelRequestGuard = useRef(new RequestGuard()).current;

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

  // Fetch entities from the backend API
  const fetchEntities = useCallback(
    async (
      showLoadingSpinner = true,
      fitAfter = false,
      afterMutation?: 'created' | 'updated' | 'deleted'
    ) => {
      if (showLoadingSpinner) {
        setIsLoading(true);
      }
      setListError(null);

      try {
        const data = await entitiesApi.getAll();
        setEntities(data);
        if (fitAfter || showLoadingSpinner) {
          setShouldFitBounds(true);
        }
      } catch (err) {
        const message =
          err instanceof ApiError
            ? err.message
            : err instanceof Error
            ? err.message
            : 'Failed to communicate with GeoEntity API';
        if (afterMutation) {
          setListError(null);
          addToast(
            'error',
            `The entity was ${afterMutation} successfully, but the list could not be refreshed. Your change is saved. ${message}`,
            'Refresh Failed'
          );
        } else {
          setListError(message);
          addToast('error', message, 'Connection Error');
        }
      } finally {
        if (showLoadingSpinner) {
          setIsLoading(false);
        }
      }
    },
    [addToast]
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

  // Initial load
  useEffect(() => {
    fetchEntities(true, true);
    checkHealth();
  }, [fetchEntities, checkHealth]);

  // Counts for header stats
  const { activeCount, maintenanceCount, inactiveCount } = useMemo(() => {
    let active = 0;
    let maintenance = 0;
    let inactive = 0;
    for (const e of entities) {
      if (e.status === 'active') active++;
      else if (e.status === 'maintenance') maintenance++;
      else if (e.status === 'inactive') inactive++;
    }
    return { activeCount: active, maintenanceCount: maintenance, inactiveCount: inactive };
  }, [entities]);

  // Handlers for selection
  const handleSelectEntity = useCallback((entity: Entity) => {
    panelRequestGuard.invalidate();
    setSelectedEntity(entity);
    setSidePanelMode('detail');
    setIsSelectingLocation(false);
    setDraftLocation(null);
  }, [panelRequestGuard]);

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
  const handleStartEdit = useCallback((entity: Entity) => {
    panelRequestGuard.invalidate();
    setSelectedEntity(entity);
    setSidePanelMode('edit');
    setIsSelectingLocation(false);
    setDraftLocation({ lat: entity.latitude, lng: entity.longitude });
    setFormBackendError(null);
    setFormBackendField(undefined);
  }, [panelRequestGuard]);

  const handleCenterMap = useCallback((entity: Entity) => {
    panelRequestGuard.invalidate();
    setSelectedEntity(entity);
    setCenterRequest((request) => request + 1);
  }, [panelRequestGuard]);

  // Location picking on map
  const handleLocationPicked = useCallback((lat: number, lng: number) => {
    setDraftLocation({ lat, lng });
    addToast('info', `Picked: ${lat.toFixed(5)}, ${lng.toFixed(5)}`, 'Location Selected');
  }, [addToast]);

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
        setEntities((current) => upsertEntity(current, created));
        if (panelRequestGuard.isCurrent(request)) {
          setSelectedEntity(created);
          setSidePanelMode('detail');
          setIsSelectingLocation(false);
          setDraftLocation(null);
        }
        await fetchEntities(false, false, 'created');
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
    [addToast, fetchEntities, panelRequestGuard]
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
        if (panelRequestGuard.isCurrent(request)) {
          setSelectedEntity((current) => current?.id === updatedEntityId ? updated : current);
          setSidePanelMode((current) => current === 'edit' ? 'detail' : current);
          setIsSelectingLocation(false);
          setDraftLocation(null);
        }
        await fetchEntities(false, false, 'updated');
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
    [selectedEntity, addToast, fetchEntities, panelRequestGuard]
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
      setEntityToDelete(null);
      if (selectedEntity?.id === entityToDelete.id) {
        setSelectedEntity(null);
        setSidePanelMode('list');
      }
      await fetchEntities(false, false, 'deleted');
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
  }, [entityToDelete, selectedEntity, addToast, fetchEntities]);

  return (
    <div className="geoentity-app">
      <Header
        totalCount={entities.length}
        activeCount={activeCount}
        maintenanceCount={maintenanceCount}
        inactiveCount={inactiveCount}
        isHealthOk={isHealthOk}
        isLoading={isLoading}
        onAddClick={handleStartCreate}
        onRefreshClick={() => {
          fetchEntities(true, true);
          checkHealth();
        }}
        onManageDefinitionsClick={() => setIsDefinitionsModalOpen(true)}
        isCreateActive={sidePanelMode === 'create'}
      />

      <main className="app-main-layout">
        {/* Interactive Leaflet Map is the primary working area */}
        <section className="map-section" aria-label="Geographic Entity Map">
          <MapView
            entities={entities}
            selectedEntity={selectedEntity}
            centerRequest={centerRequest}
            draftLocation={draftLocation}
            isSelectingLocation={isSelectingLocation}
            onSelectEntity={handleSelectEntity}
            onLocationSelected={handleLocationPicked}
            onDraftDragEnd={handleDraftDragEnd}
            shouldFitBounds={shouldFitBounds}
            onBoundsFitted={() => setShouldFitBounds(false)}
            onViewDetailsClick={handleSelectEntity}
            onEditClick={handleStartEdit}
          />
        </section>

        {/* Side Panel for list, details, and forms */}
        <aside className="side-panel-section" aria-label="Entity Management Panel">
          {sidePanelMode === 'list' && (
            <EntityList
              entities={entities}
              selectedEntity={selectedEntity}
              isLoading={isLoading}
              error={listError}
              onSelectEntity={handleSelectEntity}
              onAddNewClick={handleStartCreate}
              onRetry={() => {
                fetchEntities(true, true);
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

      {/* Notification Toast Stack */}
      <ToastContainer toasts={toasts} onDismiss={removeToast} />
    </div>
  );
}
