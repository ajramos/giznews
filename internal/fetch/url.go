package fetch

import (
	"net/url"
	"strings"
)

// trackingParams are dropped when normalizing URLs for dedup.
var trackingParams = map[string]bool{
	"utm_source": true, "utm_medium": true, "utm_campaign": true,
	"utm_term": true, "utm_content": true, "utm_id": true, "utm_reader": true,
	"fbclid": true, "gclid": true, "mc_cid": true, "mc_eid": true,
}

// NormalizeURL canonicalizes a URL so the same story pointed at with different
// tracking parameters is recognized as one.
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	// Remove tracking params in place.
	q := u.Query()
	for k := range q {
		if trackingParams[strings.ToLower(k)] {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()

	out := u.String()
	out = strings.TrimSuffix(out, "/")
	return out
}
