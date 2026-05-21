package mysqlconn_test

import (
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
)

func TestDSN(t *testing.T) {
	dsn := mysqlconn.DSN(appconfig.MySQLConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		Username: "sync_user",
		Password: "secret",
		Database: "scada_edge",
	})

	for _, want := range []string{"sync_user:secret@", "tcp(127.0.0.1:3306)", "/scada_edge", "parseTime=true"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("expected DSN to contain %q, got %q", want, dsn)
		}
	}
}
