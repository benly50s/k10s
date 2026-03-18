package deps_test

import (
	"testing"

	"github.com/benly/k10s/internal/deps"
)

func TestCheck_Structure(t *testing.T) {
	result := deps.Check()

	if len(result.Deps) != 3 {
		t.Errorf("expected 3 deps, got %d", len(result.Deps))
	}

	wantNames := []string{"kubectl", "k9s", "kubectl-oidc_login"}
	for i, want := range wantNames {
		if i >= len(result.Deps) {
			t.Errorf("missing dep at index %d (want %q)", i, want)
			continue
		}
		if result.Deps[i].Name != want {
			t.Errorf("Deps[%d].Name = %q; want %q", i, result.Deps[i].Name, want)
		}
	}

	// Verify Required flags
	type requiredEntry struct {
		name     string
		required bool
	}
	wantRequired := []requiredEntry{
		{"kubectl", true},
		{"k9s", true},
		{"kubectl-oidc_login", false},
	}
	for _, want := range wantRequired {
		for _, d := range result.Deps {
			if d.Name == want.name {
				if d.Required != want.required {
					t.Errorf("dep %q Required=%v; want %v", d.Name, d.Required, want.required)
				}
				break
			}
		}
	}
}

func TestCheck_OKReflectsRequired(t *testing.T) {
	result := deps.Check()

	allRequiredFound := true
	for _, d := range result.Deps {
		if d.Required && !d.Found {
			allRequiredFound = false
			break
		}
	}

	if result.OK != allRequiredFound {
		t.Errorf("result.OK = %v; want %v (based on required deps found status)", result.OK, allRequiredFound)
	}
}

func TestPrintReport_NoPanic(t *testing.T) {
	result := deps.CheckResult{
		Deps: []deps.Dep{},
		OK:   true,
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintReport panicked: %v", r)
		}
	}()
	deps.PrintReport(result)
}

func TestPrintReport_MixedDeps(t *testing.T) {
	result := deps.CheckResult{
		Deps: []deps.Dep{
			{Name: "kubectl", Required: true, Found: true},
			{Name: "k9s", Required: true, Found: false},
			{Name: "kubectl-oidc_login", Required: false, Found: false},
		},
		OK: false,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintReport panicked: %v", r)
		}
	}()
	deps.PrintReport(result)

	if result.OK {
		t.Error("expected result.OK=false when a required dep is missing")
	}
}
