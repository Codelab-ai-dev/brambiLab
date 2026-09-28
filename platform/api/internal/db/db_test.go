package db_test

import (
	"strings"
	"testing"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/db"
)

// Characters that break naive URI interpolation of a password.
const reservedPassword = `p@ss/w#rd:?&%=+ 'x"`

func setPGEnv(t *testing.T, host, port, user, password, database string) {
	t.Helper()
	t.Setenv("PGHOST", host)
	t.Setenv("PGPORT", port)
	t.Setenv("PGUSER", user)
	t.Setenv("PGPASSWORD", password)
	t.Setenv("PGDATABASE", database)
	t.Setenv("PGSSLMODE", "disable")
}

func TestConfigReadsReservedPasswordVerbatim(t *testing.T) {
	setPGEnv(t, "postgres", "5432", "brambilab", reservedPassword, "brambilab")
	cfg, err := db.Config("")
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.ConnConfig
	if c.Password != reservedPassword || c.Host != "postgres" || c.User != "brambilab" || c.Database != "brambilab" {
		t.Fatalf("unexpected config: host=%q user=%q db=%q password verbatim=%v",
			c.Host, c.User, c.Database, c.Password == reservedPassword)
	}
}

func TestConfigRequiresSomeSource(t *testing.T) {
	t.Setenv("PGHOST", "")
	if _, err := db.Config(""); err == nil {
		t.Fatal("expected an error without PGHOST or DATABASE_URL")
	}
}

func TestInvalidURLErrorDoesNotLeakPassword(t *testing.T) {
	_, err := db.Config("postgres://user:" + reservedPassword + "@host:notaport/db")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if strings.Contains(err.Error(), "p@ss") {
		t.Fatalf("error leaks password: %v", err)
	}
}
