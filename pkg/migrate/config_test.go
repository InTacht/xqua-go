package migrate

import "testing"

func TestValidateSchemaName(t *testing.T) {
	valid := []string{"xqua_migrate", "migrations", "_meta", "m1", "a_b_c"}
	for _, name := range valid {
		if err := validateSchemaName(name); err != nil {
			t.Errorf("expected %q to be valid: %v", name, err)
		}
	}

	invalid := []string{"", "1abc", "Public", "has space", "with-dash", "quote\"inj", "drop;table"}
	for _, name := range invalid {
		if err := validateSchemaName(name); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
}

func TestApplyDefaultsSchema(t *testing.T) {
	cfg := Config{}
	cfg.applyDefaults()
	if cfg.Schema != defaultSchema {
		t.Fatalf("expected default schema %q, got %q", defaultSchema, cfg.Schema)
	}
	if cfg.AdvisoryLockKey != defaultAdvisoryLockKey {
		t.Fatalf("expected default lock key, got %d", cfg.AdvisoryLockKey)
	}
	if cfg.PollInterval != defaultPollInterval {
		t.Fatalf("expected default poll interval, got %s", cfg.PollInterval)
	}

	cfg = Config{Schema: "custom"}
	cfg.applyDefaults()
	if cfg.Schema != "custom" {
		t.Fatalf("expected schema to be preserved, got %q", cfg.Schema)
	}
}

func TestMetaVersionMatchesSteps(t *testing.T) {
	if currentMetaVersion != len(metaSteps) {
		t.Fatalf("currentMetaVersion (%d) must equal len(metaSteps) (%d)", currentMetaVersion, len(metaSteps))
	}
}
