package main

import (
	"os"
	"strconv"

	tracewaybackend "github.com/tracewayapp/traceway/backend"
)

func main() {
	port := 8082
	if v := os.Getenv("TESTSERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}

	sqlitePath := os.Getenv("TESTSERVER_SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = ":memory:"
	}

	tracewaybackend.Run(
		tracewaybackend.WithSQLitePath(sqlitePath),
		tracewaybackend.WithPort(port),
		tracewaybackend.WithDefaultUser(
			envOr("TESTSERVER_USER", "ci@traceway.local"),
			envOr("TESTSERVER_PASSWORD", "ci-smoke-password"),
		),
		tracewaybackend.WithDefaultProject(
			"CI Smoke",
			"go",
			envOr("TESTSERVER_PROJECT_TOKEN", "ci-smoke-token"),
		),
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
