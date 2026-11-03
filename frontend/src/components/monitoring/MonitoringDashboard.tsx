import { useCallback, useEffect, useState } from 'react';
import { Entity } from '../../types/entity';
import { monitoringApi } from '../../api/monitoring';
import { DeviceSummary, Observation } from '../../types/monitoring';

export function deviceFreshness(lastSeenAt: string | null, intervalSeconds: number | null, now = Date.now()): 'Fresh' | 'Stale' | 'Unknown' {
  if (!lastSeenAt || !intervalSeconds) return 'Unknown';
  return now - Date.parse(lastSeenAt) > intervalSeconds * 3000 ? 'Stale' : 'Fresh';
}

export function MonitoringDashboard({ entities, onClose }: { entities: Entity[]; onClose: () => void }) {
  const [entityId, setEntityId] = useState('');
  const [rows, setRows] = useState<Observation[]>([]);
  const [latestRows, setLatestRows] = useState<Observation[]>([]);
  const [cursor, setCursor] = useState('');
  const [sensorId, setSensorId] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [devices, setDevices] = useState<DeviceSummary[]>([]);

  const load = useCallback(async (reset = true) => {
    if (!entityId) return;
    if (Boolean(from) !== Boolean(to)) {
      setError('Choose both a start and end time, or clear both filters.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const page = await monitoringApi.history(entityId, {
        sensorId: sensorId || undefined,
        from: from ? new Date(from).toISOString() : undefined,
        to: to ? new Date(to).toISOString() : undefined,
        cursor: reset ? undefined : cursor,
      });
      setRows((current) => reset ? page.data : [...current, ...page.data]);
      setCursor(page.next_cursor);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unable to load observations');
    } finally {
      setLoading(false);
    }
  }, [entityId, sensorId, from, to, cursor]);

  const loadLatest = useCallback(async () => {
    if (!entityId) return;
    try {
      const page = await monitoringApi.latest(entityId);
      setLatestRows(page.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unable to load latest readings');
    }
  }, [entityId]);

  useEffect(() => { monitoringApi.devices().then((page) => setDevices(page.data)).catch(() => setDevices([])); }, []);
  useEffect(() => { setLatestRows([]); void loadLatest(); }, [loadLatest]);
  useEffect(() => { setRows([]); setCursor(''); void load(true); }, [entityId, sensorId, from, to]);

  const selected = entities.find((entity) => entity.id === entityId);
  const deviceRows = devices.filter((device) => device.entity_id === entityId);
  const retry = () => { void loadLatest(); void load(true); };

  return (
    <section className="monitoring-dashboard" aria-label="IoT monitoring dashboard">
      <header><h2>Monitoring</h2><button type="button" onClick={onClose}>Back to map</button></header>
      {!entities.length ? <p>No entities are available.</p> : <label>Entity
        <select value={entityId} onChange={(event) => setEntityId(event.target.value)}>
          <option value="">Select entity</option>
          {entities.map((entity) => <option key={entity.id} value={entity.id}>{entity.name}</option>)}
        </select>
      </label>}
      {selected && <>
        <h3>{selected.name}</h3>
        <h4>Devices</h4>
        {deviceRows.length ? <ul>{deviceRows.map((device) => <li key={device.id}>
          {device.label}: {device.status}; {device.status === 'detached' ? 'Detached' : device.status === 'revoked' ? 'Revoked' : deviceFreshness(device.last_seen_at, device.expected_interval_seconds)}
        </li>)}</ul> : <p>No device is registered for this entity.</p>}
        <h4>Latest readings</h4>
        {latestRows.length ? <div className="monitoring-table-wrap"><table><thead><tr><th>Sensor</th><th>Value</th><th>Observed</th></tr></thead><tbody>
          {latestRows.map((observation) => <tr key={observation.id}><td>{observation.sensor_id}</td><td>{observation.value} {observation.unit}</td><td>{new Date(observation.observed_at).toLocaleString()}</td></tr>)}
        </tbody></table></div> : <p>No readings have been recorded for this entity.</p>}
        <form onSubmit={(event) => { event.preventDefault(); void load(true); }}>
          <label>Sensor ID <input value={sensorId} onChange={(event) => setSensorId(event.target.value)} /></label>
          <label>From <input type="datetime-local" value={from} onChange={(event) => setFrom(event.target.value)} /></label>
          <label>To <input type="datetime-local" value={to} onChange={(event) => setTo(event.target.value)} /></label>
          <button type="submit" disabled={loading}>Apply filters</button>
        </form>
        {error && <div role="alert">{error} <button type="button" onClick={retry}>Retry</button></div>}
        {loading && !rows.length && <p>Loading observations…</p>}
        {!loading && !error && !rows.length && <p>No observations in this period.</p>}
        <div className="monitoring-table-wrap"><table><thead><tr><th>Sensor</th><th>Value</th><th>Observed</th><th>Received</th></tr></thead><tbody>
          {rows.map((observation) => <tr key={observation.id}><td>{observation.sensor_id}</td><td>{observation.value} {observation.unit}</td><td>{new Date(observation.observed_at).toLocaleString()}</td><td>{new Date(observation.ingested_at).toLocaleString()}</td></tr>)}
        </tbody></table></div>
        {cursor && <button type="button" disabled={loading} onClick={() => void load(false)}>Load more</button>}
      </>}
    </section>
  );
}
