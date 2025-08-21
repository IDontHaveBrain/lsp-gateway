package search

import "sort"

func SortEnhancedResults(results []EnhancedSymbolResult, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(results, func(i, j int) bool { return results[i].DisplayName < results[j].DisplayName })
	case "relevance":
		sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	case "occurrences":
		sort.Slice(results, func(i, j int) bool { return results[i].OccurrenceCount > results[j].OccurrenceCount })
	case "kind":
		sort.Slice(results, func(i, j int) bool { return results[i].Kind < results[j].Kind })
	default:
		sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	}
}

func SortOccurrenceResults(results []SCIPOccurrenceInfo, sortBy string) {
	switch sortBy {
	case "location":
		sort.Slice(results, func(i, j int) bool {
			if results[i].DocumentURI != results[j].DocumentURI {
				return results[i].DocumentURI < results[j].DocumentURI
			}
			if results[i].LineNumber != results[j].LineNumber {
				return results[i].LineNumber < results[j].LineNumber
			}
			return results[i].Occurrence.Range.Start.Character < results[j].Occurrence.Range.Start.Character
		})
	case "file":
		sort.Slice(results, func(i, j int) bool { return results[i].DocumentURI < results[j].DocumentURI })
	case "relevance":
		sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	default:
		sort.Slice(results, func(i, j int) bool {
			if results[i].DocumentURI != results[j].DocumentURI {
				return results[i].DocumentURI < results[j].DocumentURI
			}
			return results[i].LineNumber < results[j].LineNumber
		})
	}
}
