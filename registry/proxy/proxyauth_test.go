package proxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/docker/distribution/registry/client/auth/challenge"
)

func TestRealmAllowed(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		realm  string
		want   bool
	}{
		{"same host", "https://registry.example.com", "https://registry.example.com/token", true},
		{"sibling subdomain", "https://registry-1.docker.io", "https://auth.docker.io/token", true},
		{"different etld+1", "https://registry.example.com", "https://evil.com/token", false},
		{"realm is IP literal", "https://registry.example.com", "https://10.0.0.1/token", false},
		{"remote is IP literal, realm same host", "http://10.0.0.1:5000", "http://10.0.0.1:5000/token", true},
		{"remote is IP literal, realm different IP", "http://10.0.0.1", "http://10.0.0.2/token", false},
		{"realm localhost rejected", "https://registry.example.com", "http://localhost/token", false},
		{"empty realm", "https://registry.example.com", "", false},
		{"malformed realm", "https://registry.example.com", "://nope", false},
		{"single-label hostnames must match exactly", "http://registry:5000", "http://auth:5000/token", false},
		{"same FQDN different port", "https://reg.example.com:5000", "https://reg.example.com:5001/token", true},
		{"host comparison is case insensitive", "https://Registry.Example.com", "https://REGISTRY.example.COM/token", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote, err := url.Parse(tc.remote)
			if err != nil {
				t.Fatalf("invalid remote URL: %v", err)
			}
			if got := realmAllowed(remote, tc.realm); got != tc.want {
				t.Fatalf("realmAllowed(%q, %q) = %v, want %v", tc.remote, tc.realm, got, tc.want)
			}
		})
	}
}

func TestRemoteAuthChallengerRealmValidator(t *testing.T) {
	remote, err := url.Parse("https://registry-1.docker.io")
	if err != nil {
		t.Fatal(err)
	}
	r := &remoteAuthChallenger{remoteURL: *remote}

	allowed, err := url.Parse("https://auth.docker.io/token")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.realmValidator()(allowed); err != nil {
		t.Fatalf("expected sibling-subdomain realm to pass: %v", err)
	}

	denied, err := url.Parse("https://auth.evil.example/token")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.realmValidator()(denied); err == nil {
		t.Fatalf("expected off-domain realm to be rejected")
	}
}

func TestRemoteAuthChallengerChallengeManagerFiltersBearer(t *testing.T) {
	remote, err := url.Parse("https://registry-1.docker.io")
	if err != nil {
		t.Fatal(err)
	}

	base := &fakeManager{
		ch: []challenge.Challenge{
			{Scheme: "bearer", Parameters: map[string]string{"realm": "https://auth.docker.io/token"}},
			{Scheme: "bearer", Parameters: map[string]string{"realm": "https://attacker.example/token"}},
			{Scheme: "basic", Parameters: map[string]string{}},
		},
	}
	r := &remoteAuthChallenger{remoteURL: *remote, cm: base}

	got, err := r.challengeManager().GetChallenges(*remote)
	if err != nil {
		t.Fatalf("GetChallenges: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 challenges (allowed bearer + basic), got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.Scheme == "bearer" && c.Parameters["realm"] != "https://auth.docker.io/token" {
			t.Fatalf("attacker bearer realm leaked through filter: %+v", c)
		}
	}
}

type fakeManager struct {
	ch []challenge.Challenge
}

func (f *fakeManager) GetChallenges(_ url.URL) ([]challenge.Challenge, error) {
	return f.ch, nil
}

func (f *fakeManager) AddResponse(_ *http.Response) error { return nil }
