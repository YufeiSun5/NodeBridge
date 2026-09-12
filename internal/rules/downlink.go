package rules

// DownlinkTargetDatabase keeps relay defaults separate from an explicit downlink mapping.
func (r SyncRule) DownlinkTargetDatabase(localDefault string) string {
	if r.Direction == DirectionServerToEdge && r.TargetDatabaseName != "" {
		return r.TargetDatabaseName
	}
	if localDefault != "" {
		return localDefault
	}
	if r.TargetDatabaseName != "" {
		return r.TargetDatabaseName
	}
	return r.DatabaseName
}
