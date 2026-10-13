package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleVagrantMachineReadable = `1234567890,default,version-installed,2.3.4
1234567890,default,version-latest,2.4.0
`

func TestVagrantProvider_Name(t *testing.T) {
	p := provider.NewVagrantProvider(
		config.VagrantConfig{Enabled: true, Notify: false},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	if p.Name() != "vagrant" {
		t.Errorf("expected %q, got %q", "vagrant", p.Name())
	}
	if p.DisplayName() != "Vagrant" {
		t.Errorf("expected %q, got %q", "Vagrant", p.DisplayName())
	}
}

func TestVagrantProvider_DependsOn(t *testing.T) {
	p := provider.NewVagrantProvider(
		config.VagrantConfig{Enabled: true},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseVagrantVersion(t *testing.T) {
	installed, latest := provider.ParseVagrantVersion(sampleVagrantMachineReadable)
	if installed != "2.3.4" {
		t.Errorf("expected installed 2.3.4, got %q", installed)
	}
	if latest != "2.4.0" {
		t.Errorf("expected latest 2.4.0, got %q", latest)
	}
}

func TestParseVagrantVersion_AlreadyCurrent(t *testing.T) {
	output := "123,default,version-installed,2.4.0\n123,default,version-latest,2.4.0\n"
	installed, latest := provider.ParseVagrantVersion(output)
	if installed != latest {
		t.Errorf("expected installed==latest, got installed=%q latest=%q", installed, latest)
	}
}

func TestVagrantProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("vagrant")
	if err != nil {
		t.Fatalf("vagrant not registered: %v", err)
	}
	if p.Name() != "vagrant" {
		t.Errorf("expected vagrant, got %s", p.Name())
	}
}

// fakeVagrant puts a `vagrant` script on PATH that prints versionOut and
// pluginsOut, records `plugin update` args, and exits updateExit for them.
// It returns the path of the args log.
func fakeVagrant(t *testing.T, versionOut, pluginsOut string, updateExit int) string {
	t.Helper()
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "update-args")
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("version.out", versionOut)
	write("plugins.out", pluginsOut)
	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2" in
  "version --machine-readable") cat %[1]q/version.out ;;
  "plugin list") cat %[1]q/plugins.out ;;
  "plugin update") echo "$@" > %[2]q; exit %[3]d ;;
  *) exit 1 ;;
esac
`, dir, argsLog, updateExit)
	if err := os.WriteFile(filepath.Join(dir, "vagrant"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsLog
}

// fakeRubyGems serves latest.json for the given gem → version map; other
// gems get RubyGems' "unknown" answer.
func fakeRubyGems(t *testing.T, latest map[string]string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/versions/"), "/latest.json")
		v, ok := latest[name]
		if !ok {
			v = "unknown"
		}
		fmt.Fprintf(w, `{"version":%q}`, v)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// newTestVagrant returns a provider that logs to a temp dir and whose
// notifications go to a failing fake osascript, so none reach the desktop.
func newTestVagrant(t *testing.T, gemsURL string) *provider.VagrantProvider {
	t.Helper()
	failingCommands(t, "osascript")
	logger := logging.New(t.TempDir(), logging.ParseLevel("debug"))
	t.Cleanup(func() { _ = logger.Close() })
	p := provider.NewVagrantProvider(
		config.VagrantConfig{Enabled: true, Notify: true},
		config.NotificationsConfig{Enabled: true, Tool: "osascript"},
		logger,
	)
	p.SetGemsURL(gemsURL)
	return p
}

func TestParseVagrantPluginList(t *testing.T) {
	out := `vagrant-vbguest (0.31.0, global)
  - Version Constraint: > 0
vagrant-share (2.0.0, system)
vagrant-hostsupdater (1.2.5, global)
`
	want := []provider.OutdatedItem{
		{Name: "vagrant-vbguest", CurrentVersion: "0.31.0"},
		{Name: "vagrant-hostsupdater", CurrentVersion: "1.2.5"},
	}
	if got := provider.ParseVagrantPluginList(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := provider.ParseVagrantPluginList("No plugins installed.\n"); len(got) != 0 {
		t.Errorf("expected no plugins, got %+v", got)
	}
}

func TestVagrantScan_NothingOutdated(t *testing.T) {
	fakeVagrant(t, "1,default,version-installed,2.4.0\n1,default,version-latest,2.4.0\n", "No plugins installed.\n", 0)
	p := newTestVagrant(t, fakeRubyGems(t, nil))

	res := p.Scan(context.Background())
	if !res.Available || res.Error != nil || len(res.Outdated) != 0 {
		t.Errorf("expected nothing outdated, got %+v", res)
	}
}

func TestVagrantScan_ReportsOnlyOutdatedPlugins(t *testing.T) {
	fakeVagrant(t, sampleVagrantMachineReadable, `vagrant-vbguest (0.31.0, global)
vagrant-hostsupdater (1.2.5, global)
local-plugin (0.1.0, global)
vagrant-share (2.0.0, system)
`, 0)
	p := newTestVagrant(t, fakeRubyGems(t, map[string]string{
		"vagrant-vbguest":      "0.32.0",
		"vagrant-hostsupdater": "1.2.5",
	}))

	res := p.Scan(context.Background())
	want := []provider.OutdatedItem{
		{Name: "vagrant", CurrentVersion: "2.3.4", LatestVersion: "2.4.0"},
		{Name: "vagrant-vbguest", CurrentVersion: "0.31.0", LatestVersion: "0.32.0"},
	}
	if res.Error != nil || !reflect.DeepEqual(res.Outdated, want) {
		t.Errorf("got %+v (err %v), want %+v", res.Outdated, res.Error, want)
	}
}

func TestVagrantScan_PluginListFailureIsReported(t *testing.T) {
	failingCommands(t, "vagrant")
	p := newTestVagrant(t, fakeRubyGems(t, nil))

	if res := p.Scan(context.Background()); res.Error == nil {
		t.Errorf("expected scan error, got %+v", res)
	}
}

func TestVagrantScan_NotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := provider.NewVagrantProvider(config.VagrantConfig{}, config.NotificationsConfig{}, nil)

	if res := p.Scan(context.Background()); res.Available {
		t.Errorf("expected unavailable, got %+v", res)
	}
}

func TestVagrantScan_UnreachableLookupsSkipPlugins(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()
	garbled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer garbled.Close()
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	for name, url := range map[string]string{
		"server error": down.URL,
		"bad json":     garbled.URL,
		"no server":    closed.URL,
		"invalid url":  "\x7f",
	} {
		t.Run(name, func(t *testing.T) {
			fakeVagrant(t, "", "vagrant-vbguest (0.31.0, global)\n", 0)
			res := newTestVagrant(t, url).Scan(context.Background())
			if res.Error != nil || len(res.Outdated) != 0 {
				t.Errorf("expected plugin skipped, got %+v", res)
			}
		})
	}
}

func TestVagrantUpdate_UpdatesOutdatedPlugins(t *testing.T) {
	for _, tc := range []struct {
		name       string
		exit       int
		wantUpdate bool
	}{{"success", 0, true}, {"failure", 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			argsLog := fakeVagrant(t, "", "", tc.exit)
			p := newTestVagrant(t, "")

			res := p.Update(context.Background(), []provider.OutdatedItem{
				{Name: "vagrant", CurrentVersion: "2.3.4", LatestVersion: "2.4.0"},
				{Name: "vagrant-vbguest"},
				{Name: "vagrant-hostsupdater"},
			})

			args, _ := os.ReadFile(argsLog)
			if got := strings.TrimSpace(string(args)); got != "plugin update vagrant-vbguest vagrant-hostsupdater" {
				t.Errorf("unexpected update args %q", got)
			}
			plugins := []string{"vagrant-vbguest", "vagrant-hostsupdater"}
			got := res.Failed
			if tc.wantUpdate {
				got = res.Updated
			}
			if !reflect.DeepEqual(got, plugins) {
				t.Errorf("got %+v, want plugins %v", res, plugins)
			}
		})
	}
}

func TestVagrantUpdate_BinaryOnlySkipsPluginUpdate(t *testing.T) {
	argsLog := fakeVagrant(t, "", "", 0)
	res := newTestVagrant(t, "").Update(context.Background(), []provider.OutdatedItem{
		{Name: "vagrant", CurrentVersion: "2.3.4", LatestVersion: "2.4.0"},
	})
	if _, err := os.Stat(argsLog); err == nil {
		t.Error("plugin update ran with no outdated plugins")
	}
	if len(res.Updated)+len(res.Failed) != 0 {
		t.Errorf("unexpected result %+v", res)
	}
}
