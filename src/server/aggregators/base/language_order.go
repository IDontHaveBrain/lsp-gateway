package base

// OrderLanguagesByPriority returns languages from the results map ordered by the given priority list.
// Languages in the priority list appear first (in priority order), followed by any remaining languages.
func OrderLanguagesByPriority[V any](results map[string]V, priority []string) []string {
	if len(priority) == 0 {
		langs := make([]string, 0, len(results))
		for lang := range results {
			langs = append(langs, lang)
		}
		return langs
	}

	ordered := make([]string, 0, len(results))
	seen := make(map[string]bool)

	for _, priorityLang := range priority {
		if _, exists := results[priorityLang]; exists {
			ordered = append(ordered, priorityLang)
			seen[priorityLang] = true
		}
	}

	for lang := range results {
		if !seen[lang] {
			ordered = append(ordered, lang)
		}
	}

	return ordered
}
