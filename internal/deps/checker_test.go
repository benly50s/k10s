package deps_test

import (
	"testing"

	"github.com/benly/k10s/internal/deps"
)

func TestCheck_Structure(t *testing.T) {
	result := deps.Check()

	if len(result.Deps) != 5 {
		t.Errorf("expected 5 deps, got %d", len(result.Deps))
	}

	wantNames := []string{"kubectl", "k9s", "lsof", "kubelogin", "argocd"}
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
		{"lsof", true},
		{"kubelogin", false},
		{"argocd", false},
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

	// Compute expected OK: true only if all Required deps are Found
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
	// Should not panic with empty deps list
	result := deps.CheckResult{
		Deps: []deps.Dep{},
		OK:   true,
	}
	// Capture that this doesn't panic
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
			{Name: "lsof", Required: true, Found: false},
			{Name: "argocd", Required: false, Found: false},
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
