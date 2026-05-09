package challenge

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

type stubManager struct {
	challenges []Challenge
	getErr     error
	addCalls   int
}

func (s *stubManager) GetChallenges(_ url.URL) ([]Challenge, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.challenges, nil
}

func (s *stubManager) AddResponse(_ *http.Response) error {
	s.addCalls++
	return nil
}

func TestFilteringManagerNilKeepReturnsBase(t *testing.T) {
	base := &stubManager{}
	if got := NewFilteringManager(base, nil); got != base {
		t.Fatalf("nil keep should return the base manager unchanged, got %T", got)
	}
}

func TestFilteringManagerFiltersOnRead(t *testing.T) {
	base := &stubManager{
		challenges: []Challenge{
			{Scheme: "bearer", Parameters: map[string]string{"realm": "https://auth.example.com/token"}},
			{Scheme: "bearer", Parameters: map[string]string{"realm": "https://attacker.example/token"}},
			{Scheme: "basic"},
		},
	}

	keep := func(c Challenge) bool {
		return c.Scheme != "bearer" || c.Parameters["realm"] == "https://auth.example.com/token"
	}

	endpoint, _ := url.Parse("https://registry.example.com/v2/")
	got, err := NewFilteringManager(base, keep).GetChallenges(*endpoint)
	if err != nil {
		t.Fatalf("GetChallenges: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 surviving challenges (allowed bearer + basic), got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.Scheme == "bearer" && c.Parameters["realm"] != "https://auth.example.com/token" {
			t.Fatalf("attacker realm leaked: %+v", c)
		}
	}
}

func TestFilteringManagerPropagatesGetError(t *testing.T) {
	wantErr := errors.New("boom")
	base := &stubManager{getErr: wantErr}

	endpoint, _ := url.Parse("https://registry.example.com/v2/")
	_, err := NewFilteringManager(base, func(Challenge) bool { return true }).GetChallenges(*endpoint)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected underlying error to propagate, got %v", err)
	}
}

func TestFilteringManagerAddResponsePassesThrough(t *testing.T) {
	base := &stubManager{}
	m := NewFilteringManager(base, func(Challenge) bool { return false })

	req, _ := http.NewRequest("GET", "https://registry.example.com/v2/", nil)
	resp := &http.Response{Request: req, Header: make(http.Header)}
	if err := m.AddResponse(resp); err != nil {
		t.Fatalf("AddResponse: %v", err)
	}
	if base.addCalls != 1 {
		t.Fatalf("expected AddResponse to forward exactly once, got %d", base.addCalls)
	}
}
