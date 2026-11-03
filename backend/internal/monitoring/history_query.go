package monitoring

// HistoryQuery builds the production sensor-history query and its bound values.
// Optional filters are represented by omitted predicates so SQLite can use the
// matching entity/time or entity/sensor/time index.
func HistoryQuery(entityID, sensorID, from, to, cursorTime, cursorID string, limit int) (string, []any) {
	query := `SELECT id,entity_id,device_id,sensor_id,value,unit,observed_at,ingested_at FROM sensor_observations WHERE entity_id=? AND observed_at>=? AND observed_at<=?`
	args := []any{entityID, from, to}
	if sensorID != "" {
		query += ` AND sensor_id=?`
		args = append(args, sensorID)
	}
	if cursorTime != "" {
		query += ` AND (observed_at<? OR (observed_at=? AND id<?))`
		args = append(args, cursorTime, cursorTime, cursorID)
	}
	query += ` ORDER BY observed_at DESC,id DESC LIMIT ?`
	args = append(args, limit)
	return query, args
}
