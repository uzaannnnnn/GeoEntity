import { apiClient } from './client';
import { DeviceSummary, ObservationPage } from '../types/monitoring';

export const monitoringApi = {
  devices: (limit=100) => apiClient.get<{data:DeviceSummary[];next_cursor:string}>(`/admin/devices?limit=${limit}`),
  latest: (entityId:string) => apiClient.get<ObservationPage>(`/monitoring/latest?entity_id=${encodeURIComponent(entityId)}&limit=100`),
  history: (entityId:string, options:{from?:string;to?:string;sensorId?:string;cursor?:string;limit?:number}) => {
    const query=new URLSearchParams(); if(options.from)query.set('from',options.from);if(options.to)query.set('to',options.to);if(options.sensorId)query.set('sensor_id',options.sensorId);if(options.cursor)query.set('cursor',options.cursor);query.set('limit',String(options.limit??50));
    return apiClient.get<ObservationPage>(`/entities/${encodeURIComponent(entityId)}/sensors/observations?${query.toString()}`);
  },
};
