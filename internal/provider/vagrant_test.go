package provider_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
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
