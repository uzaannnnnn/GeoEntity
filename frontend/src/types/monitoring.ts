export interface DeviceSummary { id:string; entity_id:string|null; label:string; status:'active'|'revoked'|'detached'; created_at:string; last_seen_at:string|null; expected_interval_seconds:number|null }
export interface Observation { id:string; entity_id:string; device_id:string; sensor_id:string; value:number; unit:string; observed_at:string; ingested_at:string }
export interface ObservationPage { data:Observation[]; next_cursor:string }
