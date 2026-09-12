package mysqlconn_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/go-sql-driver/mysql"
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
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, rawQuery, _ := strings.Cut(dsn, "?")
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InterpolateParams || params.Get("charset") != "utf8mb4" || cfg.MultiStatements {
		t.Fatal("expected UTF-8 driver interpolation with multi-statements disabled")
	}
}
