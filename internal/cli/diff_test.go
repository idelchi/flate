package cli

import (
	"strings"
	"testing"

	"github.com/home-operations/flate/pkg/diff"
	"github.com/home-operations/flate/pkg/manifest"
	"github.com/home-operations/flate/pkg/orchestrator"
)

func TestOmittedValuesNotes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      string
		resource  string
		namespace string
		category  string
		wantNote  bool
	}{
		{name: "all", category: manifest.WarnOmittedValues, wantNote: true},
		{name: "matching filters", kind: manifest.KindHelmRelease, resource: "demo", namespace: "apps", category: manifest.WarnOmittedValues, wantNote: true},
		{name: "other kind", kind: manifest.KindKustomization, category: manifest.WarnOmittedValues},
		{name: "other name", resource: "other", category: manifest.WarnOmittedValues},
		{name: "other namespace", namespace: "other", category: manifest.WarnOmittedValues},
		{name: "other warning", category: manifest.WarnStaleValues},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := orchestrator.New(orchestrator.Config{Path: t.TempDir(), CacheDir: t.TempDir()})
			if err != nil {
				t.Fatalf("New orchestrator: %v", err)
			}
			res := &orchestrator.Result{Warnings: []manifest.Warning{{
				Resource: manifest.NamedResource{Kind: manifest.KindHelmRelease, Namespace: "apps", Name: "demo"},
				Category: tc.category,
				Message:  "required valuesFrom ConfigMap/apps/values omitted from offline render",
			}}}
			got := omittedValuesNotes(diffSide{O: o, Res: res}, &commonFlags{namespace: tc.namespace}, tc.kind, tc.resource, diff.SideCurrent)
			if (len(got) == 1) != tc.wantNote {
				t.Fatalf("notes = %v, wantNote = %v", got, tc.wantNote)
			}
			if tc.wantNote && (!strings.Contains(got[0], "HelmRelease apps/demo: incomplete comparison on the current side") ||
				!strings.Contains(got[0], "ConfigMap/apps/values")) {
				t.Errorf("note must identify the release, side and omitted reference: %q", got[0])
			}
		})
	}
}
