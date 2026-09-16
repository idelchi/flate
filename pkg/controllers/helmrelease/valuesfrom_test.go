package helmrelease

import (
	"context"
	"testing"

	"github.com/home-operations/flate/internal/testutil"
	"github.com/home-operations/flate/pkg/manifest"
	"github.com/home-operations/flate/pkg/store"
)

func TestController_OmittedValuesAfterWait(t *testing.T) {
	for _, tc := range []struct {
		name         string
		waitValues   bool
		materialized bool
		wantGreeting string
		wantWarnings int
	}{
		{name: "values wait, still missing", waitValues: true, wantGreeting: "fallback", wantWarnings: 1},
		{name: "values wait, materialized", waitValues: true, materialized: true, wantGreeting: "supplied"},
		{name: "chart wait, still missing", wantGreeting: "fallback", wantWarnings: 1},
		{name: "chart wait, materialized", materialized: true, wantGreeting: "supplied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			testutil.WriteFile(t, dir, "mychart/Chart.yaml", "apiVersion: v2\nname: mychart\nversion: 0.1.0\n")
			testutil.WriteFile(t, dir, "mychart/templates/configmap.yaml", `apiVersion: v1
kind: ConfigMap
metadata: {name: demo}
data:
  greeting: {{ .Values.greeting | default "fallback" | quote }}
`)
			c, st := newTestControllerWithOptions(t, ReconcileOptions{})
			src := &manifest.GitRepository{Name: "charts", Namespace: "flux-system"}
			st.AddObject(src)
			st.SetArtifact(src.Named(), &store.SourceArtifact{
				Kind: manifest.KindGitRepository, URL: "file://" + dir, LocalPath: dir,
			})
			st.UpdateStatus(src.Named(), store.StatusPending, "fetching")
			if tc.waitValues {
				st.UpdateStatus(src.Named(), store.StatusReady, "")
			}
			st.AddObject(&manifest.RawObject{
				Kind: "ExternalSecret", APIVersion: "external-secrets.io/v1",
				Name: "app-creds", Namespace: "default",
				Spec: map[string]any{"target": map[string]any{"name": "app-values"}},
			})
			hr := &manifest.HelmRelease{
				Name: "demo", Namespace: "default",
				ValuesFrom: []manifest.ValuesReference{{Kind: manifest.KindSecret, Name: "app-values"}},
				Chart: manifest.HelmChart{
					Name: "mychart", RepoName: "charts",
					RepoNamespace: "flux-system", RepoKind: manifest.KindGitRepository,
				},
			}
			if tc.waitValues {
				hr.ValuesFrom = append(hr.ValuesFrom, manifest.ValuesReference{Kind: manifest.KindConfigMap, Name: "chart-values"})
			}
			st.AddObject(hr)
			if blocked := c.ReconcileNode(context.Background(), hr.Named(), 0); len(blocked) == 0 {
				t.Fatal("expected reconciliation to wait for a dependency")
			}
			if warnings := st.Warnings(); len(warnings) != 0 {
				t.Errorf("unresolved wait must not record omissions: %+v", warnings)
			}
			st.AddObject(&manifest.ConfigMap{
				Name: "chart-values", Namespace: "default",
				Data: map[string]any{"values.yaml": "{}"},
			})
			if tc.materialized {
				st.AddObject(&manifest.Secret{
					Name: "app-values", Namespace: "default",
					StringData: map[string]any{"values.yaml": "greeting: supplied\n"},
				})
			}
			st.UpdateStatus(src.Named(), store.StatusReady, "")
			if info := dispatchToFixpoint(t, c, st, hr.Named()); info.Status != store.StatusReady {
				t.Fatalf("status = %+v, want StatusReady", info)
			}
			art, ok := st.GetArtifact(hr.Named()).(*store.HelmReleaseArtifact)
			if !ok {
				t.Fatal("HelmRelease artifact was not written")
			}
			if got := renderedConfigMapValue(art.Manifests, "greeting"); got != tc.wantGreeting {
				t.Errorf("greeting = %q, want %q", got, tc.wantGreeting)
			}
			if warnings := st.Warnings(); len(warnings) != tc.wantWarnings {
				t.Errorf("warnings = %+v, want %d", warnings, tc.wantWarnings)
			}
		})
	}
}
