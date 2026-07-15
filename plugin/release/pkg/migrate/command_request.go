package migrate

type migrationCommandRequest struct {
	startDirectory string
	preview        bool
}

func parseMigrationCommandRequest(flags map[string]any, startDirectory string) migrationCommandRequest {
	preview, _ := flags["dry-run"].(bool)
	return migrationCommandRequest{
		startDirectory: startDirectory,
		preview:        preview,
	}
}
