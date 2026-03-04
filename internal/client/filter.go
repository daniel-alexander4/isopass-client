package client

import "strings"

// FilterSecrets filters secrets by comma-separated search terms and an optional type filter.
// All terms must match against tags or non-secret field values (case-insensitive, partial match).
func FilterSecrets(secrets []Secret, terms string, typeFilter string) []Secret {
	searchTerms := parseTerms(terms)
	var result []Secret
	for _, s := range secrets {
		if typeFilter != "" && s.Type != typeFilter {
			continue
		}
		if len(searchTerms) > 0 && !matchesAllTerms(s, searchTerms) {
			continue
		}
		result = append(result, s)
	}
	return result
}

func parseTerms(raw string) []string {
	var terms []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			terms = append(terms, t)
		}
	}
	return terms
}

func matchesAllTerms(s Secret, terms []string) bool {
	searchable := make([]string, 0, len(s.Tags)+4)
	for _, tag := range s.Tags {
		searchable = append(searchable, strings.ToLower(tag))
	}
	fields := ParseFields(s.Type, s.Value)
	for _, f := range fields {
		if !f.Secret {
			searchable = append(searchable, strings.ToLower(f.Value))
		}
	}
	for _, term := range terms {
		found := false
		for _, s := range searchable {
			if strings.Contains(s, term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
