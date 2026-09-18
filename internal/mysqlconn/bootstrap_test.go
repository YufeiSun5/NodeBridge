package mysqlconn

import (
	"context"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
)

func TestSystemDatabaseIdentifier(t *testing.T) {
	for _, name := range []string{"", "db`", "db;DROP DATABASE mysql", "db/name", "db\nname", "mysql.db", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if err := CreateSystemDatabase(context.Background(), appconfig.MySQLConfig{Database: name}); err == nil {
			t.Fatalf("accepted unsafe database %q", name)
		}
	}
	for _, name := range []string{"scada_center", "scada_edge", "nodebridge-new", "nb_gen_001"} {
		if !databaseIdentifier.MatchString(name) {
			t.Fatalf("rejected database %q", name)
		}
	}
}
