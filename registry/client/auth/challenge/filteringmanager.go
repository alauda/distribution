package challenge

import (
	"net/http"
	"net/url"
)

// FilteringManager wraps a Manager so callers see only challenges accepted by
// keep. It is used by the proxy to drop bearer challenges whose realm is not
// part of the configured upstream's trust boundary
// (GHSA-3p65-76g6-3w7r / CVE-2026-33540).
type FilteringManager struct {
	base Manager
	keep func(Challenge) bool
}

// NewFilteringManager returns base unchanged when keep is nil so callers can
// opt out without a separate code path.
func NewFilteringManager(base Manager, keep func(Challenge) bool) Manager {
	if keep == nil {
		return base
	}

	return FilteringManager{
		base: base,
		keep: keep,
	}
}

// GetChallenges returns only challenges for which keep returned true.
func (m FilteringManager) GetChallenges(endpoint url.URL) ([]Challenge, error) {
	challenges, err := m.base.GetChallenges(endpoint)
	if err != nil {
		return nil, err
	}

	filtered := make([]Challenge, 0, len(challenges))
	for _, c := range challenges {
		if m.keep(c) {
			filtered = append(filtered, c)
		}
	}

	return filtered, nil
}

// AddResponse forwards untouched. Filtering happens on read so the underlying
// manager keeps the original challenge stream for diagnostics.
func (m FilteringManager) AddResponse(resp *http.Response) error {
	return m.base.AddResponse(resp)
}
