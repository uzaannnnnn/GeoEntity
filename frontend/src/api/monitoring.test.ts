import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { monitoringApi } from './monitoring';

describe('monitoring API',()=>{
  beforeEach(()=>vi.restoreAllMocks());
  it('loads administrator device metadata without credential fields',async()=>{const page={data:[],next_cursor:''};vi.spyOn(apiClient,'get').mockResolvedValueOnce(page);await expect(monitoringApi.devices()).resolves.toEqual(page);expect(apiClient.get).toHaveBeenCalledWith('/admin/devices?limit=100')});
  it('encodes selected entity, time, sensor, and history cursor filters',async()=>{vi.spyOn(apiClient,'get').mockResolvedValueOnce({data:[],next_cursor:''});await monitoringApi.history('entity/a',{from:'2026-11-01T00:00:00Z',to:'2026-11-02T00:00:00Z',sensorId:'ambient_temperature',cursor:'opaque token',limit:25});expect(apiClient.get).toHaveBeenCalledWith('/entities/entity%2Fa/sensors/observations?from=2026-11-01T00%3A00%3A00Z&to=2026-11-02T00%3A00%3A00Z&sensor_id=ambient_temperature&cursor=opaque+token&limit=25')});
});
