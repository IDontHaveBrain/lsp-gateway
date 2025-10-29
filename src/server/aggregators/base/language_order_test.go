package base

import (
	"reflect"
	"testing"
)

func TestOrderLanguagesByPriority(t *testing.T) {
	tests := []struct {
		name     string
		results  map[string]int
		priority []string
		want     []string
	}{
		{
			name: "with_priority_all_present",
			results: map[string]int{
				"go":         1,
				"python":     2,
				"javascript": 3,
				"java":       4,
			},
			priority: []string{"java", "go", "python"},
			want:     []string{"java", "go", "python", "javascript"},
		},
		{
			name: "with_priority_some_present",
			results: map[string]int{
				"go":     1,
				"python": 2,
				"rust":   3,
			},
			priority: []string{"java", "go", "typescript", "python"},
			want:     []string{"go", "python", "rust"},
		},
		{
			name: "no_priority_specified",
			results: map[string]int{
				"go":     1,
				"python": 2,
			},
			priority: []string{},
			want:     []string{"go", "python"},
		},
		{
			name:     "empty_results",
			results:  map[string]int{},
			priority: []string{"go", "python"},
			want:     []string{},
		},
		{
			name: "priority_with_no_matches",
			results: map[string]int{
				"go":     1,
				"python": 2,
			},
			priority: []string{"java", "rust", "csharp"},
			want:     []string{"go", "python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrderLanguagesByPriority(tt.results, tt.priority)

			if len(got) != len(tt.want) {
				t.Errorf("OrderLanguagesByPriority() length = %v, want %v", len(got), len(tt.want))
				return
			}

			gotSet := make(map[string]bool)
			wantSet := make(map[string]bool)
			for _, lang := range got {
				gotSet[lang] = true
			}
			for _, lang := range tt.want {
				wantSet[lang] = true
			}

			if !reflect.DeepEqual(gotSet, wantSet) {
				t.Errorf("OrderLanguagesByPriority() languages mismatch\ngot:  %v\nwant: %v", got, tt.want)
				return
			}

			priorityIndex := make(map[string]int)
			for i, lang := range tt.priority {
				priorityIndex[lang] = i
			}

			for i := 0; i < len(got)-1; i++ {
				curr := got[i]
				next := got[i+1]

				currInPriority := false
				nextInPriority := false
				currIdx := -1
				nextIdx := -1

				if idx, exists := priorityIndex[curr]; exists {
					currInPriority = true
					currIdx = idx
				}
				if idx, exists := priorityIndex[next]; exists {
					nextInPriority = true
					nextIdx = idx
				}

				if currInPriority && nextInPriority {
					if currIdx > nextIdx {
						t.Errorf("OrderLanguagesByPriority() priority violation: %s (idx %d) comes before %s (idx %d)", curr, currIdx, next, nextIdx)
					}
				} else if !currInPriority && nextInPriority {
					t.Errorf("OrderLanguagesByPriority() priority violation: non-priority language %s comes before priority language %s", curr, next)
				}
			}
		})
	}
}

func TestOrderLanguagesByPriority_DifferentTypes(t *testing.T) {
	t.Run("with_string_slice_values", func(t *testing.T) {
		results := map[string][]string{
			"go":     {"file1.go"},
			"python": {"file1.py"},
			"java":   {"File1.java"},
		}
		priority := []string{"python", "go"}

		got := OrderLanguagesByPriority(results, priority)

		if len(got) != 3 {
			t.Fatalf("Expected 3 languages, got %d", len(got))
		}

		if got[0] != "python" || got[1] != "go" {
			t.Errorf("Priority not respected: got %v", got)
		}
	})

	t.Run("with_map_values", func(t *testing.T) {
		results := map[string]map[string]int{
			"go":         {"count": 5},
			"typescript": {"count": 10},
			"rust":       {"count": 3},
		}
		priority := []string{"rust", "typescript", "go"}

		got := OrderLanguagesByPriority(results, priority)

		if !reflect.DeepEqual(got, []string{"rust", "typescript", "go"}) {
			t.Errorf("Expected exact priority order, got %v", got)
		}
	})
}
