package cli

import (
	"testing"

	"github.com/home-operations/flate/pkg/manifest"
	"github.com/home-operations/flate/pkg/orchestrator"
	"github.com/home-operations/flate/pkg/store"
)

func TestSuppressionReasons(t *testing.T) {
	for _, tc := range []struct {
		name      string
		namespace string
		message   string
		wantSkip  bool
	}{
		{name: "skipped", namespace: "apps", message: "skipped: missing Secret", wantSkip: true},
		{name: "outside namespace", namespace: "other", message: "skipped: missing Secret"},
		{name: "unchanged", namespace: "apps", message: store.MsgUnchanged},
		{name: "suspended", namespace: "apps", message: store.MsgSuspended},
		{name: "ready", namespace: "apps", message: "rendered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := orchestrator.New(orchestrator.Config{Path: t.TempDir(), CacheDir: t.TempDir()})
			if err != nil {
				t.Fatalf("New orchestrator: %v", err)
			}
			objects := []manifest.BaseManifest{
				&manifest.Kustomization{Name: "apps", Namespace: tc.namespace},
				&manifest.HelmRelease{Name: "demo", Namespace: tc.namespace},
			}
			for _, obj := range objects {
				o.Store().AddObject(obj)
				o.Store().UpdateStatus(obj.Named(), store.StatusReady, tc.message)
			}
			failedID := manifest.NamedResource{Kind: manifest.KindHelmRelease, Namespace: "apps", Name: "failed"}
			res := &orchestrator.Result{Failed: map[manifest.NamedResource]store.StatusInfo{
				failedID: {Status: store.StatusFailed, Message: "render failed"},
			}}
			got := suppressionReasons(o, res, &commonFlags{namespace: "apps"})
			if got[failedID] != "render failed" {
				t.Errorf("failure reason must be preserved; got %v", got)
			}
			for _, obj := range objects {
				message, ok := got[obj.Named()]
				if ok != tc.wantSkip || (ok && message != tc.message) {
					t.Errorf("%s: got %q (present=%v), want %q (present=%v)",
						obj.Named(), message, ok, tc.message, tc.wantSkip)
				}
			}
		})
	}
}
