package preflight

import (
	"errors"
	"strings"
	"testing"
)

func prodEnv() map[string]string {
	return map[string]string{
		"PUBLIC_ORIGIN": "https://brambilab.dev", "PGHOST": "postgres", "PGUSER": "brambilab", "PGDATABASE": "brambilab",
		"PGPASSWORD": "a-long-random-secret-value-42", "ADMIN_GITHUB_USER_ID": "201345228", "GITHUB_CLIENT_ID": "Ov23liRealClient",
		"GITHUB_CLIENT_SECRET": "0123456789abcdef0123456789abcdef01234567", "TRUSTED_PROXIES": "10.0.1.0/24",
		"CONTACT_ENABLED": "true", "RESEND_API_KEY": "re_live_key_123", "CONTACT_FROM": "BrambiLab <contacto@brambilab.dev>", "CONTACT_TO": "gustavo@brambilab.dev",
	}
}

func run(m map[string]string, writable error) []Result {
	return Check(func(k string) string { return m[k] }, func(string) error { return writable })
}

func TestProductionConfigPasses(t *testing.T) {
	rs := run(prodEnv(), nil)
	if Failed(rs) {
		t.Fatalf("expected pass: %+v", rs)
	}
	for _, r := range rs {
		// Values never appear in the report.
		for _, secret := range []string{"a-long-random-secret-value-42", "0123456789abcdef", "re_live_key_123", "gustavo@"} {
			if strings.Contains(r.Message, secret) {
				t.Fatalf("value leaked in %s: %s", r.Name, r.Message)
			}
		}
	}
}

func TestProductionConfigRejects(t *testing.T) {
	for name, change := range map[string]map[string]string{
		"localhost origin":     {"PUBLIC_ORIGIN": "http://localhost:8000"},
		"example origin":       {"PUBLIC_ORIGIN": "https://brambilab.example.com"},
		"weak db password":     {"PGPASSWORD": "change-me@local/#1"},
		"missing db password":  {"PGPASSWORD": ""},
		"login not configured": {"GITHUB_CLIENT_SECRET": ""},
		"e2e oauth":            {"GITHUB_CLIENT_ID": "e2e-client"},
		"fake github url":      {"GITHUB_TOKEN_URL": "http://fakegithub:9999/x"},
		"fake resend url":      {"RESEND_API_URL": "http://fakeresend:9998"},
		"contact incomplete":   {"CONTACT_TO": ""},
		"contact test values":  {"CONTACT_TO": "owner@example.test"},
		"jobs off":             {"BACKGROUND_JOBS": "off"},
	} {
		m := prodEnv()
		for k, v := range change {
			m[k] = v
		}
		if !Failed(run(m, nil)) {
			t.Errorf("%s: should fail", name)
		}
	}
	if !Failed(run(prodEnv(), errors.New("read-only file system"))) {
		t.Error("unwritable media root should fail")
	}
	// Contact disabled is fine; unset TRUSTED_PROXIES is only a warning.
	m := prodEnv()
	m["CONTACT_ENABLED"], m["TRUSTED_PROXIES"] = "false", ""
	rs := run(m, nil)
	warned := false
	for _, r := range rs {
		warned = warned || (r.Level == Warn && r.Name == "TRUSTED_PROXIES")
	}
	if Failed(rs) || !warned {
		t.Fatalf("disabled contact / proxies warning: %+v", rs)
	}
}
