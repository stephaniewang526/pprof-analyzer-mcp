package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// DetectPotentialMemoryLeaks analyzes Heap profiles and attempts to detect potential memory leaks.
// This function compares two Heap profiles (typically snapshots from different points in time) and identifies memory allocations with significant growth.
func DetectPotentialMemoryLeaks(oldProfile, newProfile *profile.Profile, threshold float64, limit int) (string, error) {
	if threshold <= 0 {
		threshold = 0.1 // Default threshold: 10% growth
	}
	if limit <= 0 {
		limit = 10 // Default: show top 10 potential leaks
	}

	// Analyze memory usage in the old profile
	oldMemory := make(map[string]int64)
	oldObjects := make(map[string]int64)

	// Find indices for inuse_space and inuse_objects
	oldValueIndex := -1
	oldObjectsIndex := -1

	for i, st := range oldProfile.SampleType {
		if st.Type == "inuse_space" && st.Unit == "bytes" {
			oldValueIndex = i
		}
		if st.Type == "inuse_objects" && st.Unit == "count" {
			oldObjectsIndex = i
		}
	}

	if oldValueIndex == -1 {
		return "", fmt.Errorf("could not find inuse_space sample type in the old profile")
	}

	// Aggregate memory usage in the old profile
	for _, s := range oldProfile.Sample {
		if len(s.Location) > 0 && len(s.Value) > oldValueIndex {
			v := s.Value[oldValueIndex]

			// Get object count
			var objCount int64 = 0
			if oldObjectsIndex >= 0 && len(s.Value) > oldObjectsIndex {
				objCount = s.Value[oldObjectsIndex]
			}

			// Extract type information (if available)
			typeName := "unknown"
			if len(s.Label) > 0 {
				if typeLabels, ok := s.Label["type"]; ok && len(typeLabels) > 0 {
					typeName = typeLabels[0]
				} else if objLabels, ok := s.Label["object"]; ok && len(objLabels) > 0 {
					typeName = objLabels[0]
				}
			}

			// Aggregate by type
			oldMemory[typeName] += v
			if objCount > 0 {
				oldObjects[typeName] += objCount
			}
		}
	}

	// Analyze memory usage in the new profile
	newMemory := make(map[string]int64)
	newObjects := make(map[string]int64)

	// Find indices for inuse_space and inuse_objects
	newValueIndex := -1
	newObjectsIndex := -1

	for i, st := range newProfile.SampleType {
		if st.Type == "inuse_space" && st.Unit == "bytes" {
			newValueIndex = i
		}
		if st.Type == "inuse_objects" && st.Unit == "count" {
			newObjectsIndex = i
		}
	}

	if newValueIndex == -1 {
		return "", fmt.Errorf("could not find inuse_space sample type in the new profile")
	}

	// Aggregate memory usage in the new profile
	for _, s := range newProfile.Sample {
		if len(s.Location) > 0 && len(s.Value) > newValueIndex {
			v := s.Value[newValueIndex]

			// Get object count
			var objCount int64 = 0
			if newObjectsIndex >= 0 && len(s.Value) > newObjectsIndex {
				objCount = s.Value[newObjectsIndex]
			}

			// Extract type information (if available)
			typeName := "unknown"
			if len(s.Label) > 0 {
				if typeLabels, ok := s.Label["type"]; ok && len(typeLabels) > 0 {
					typeName = typeLabels[0]
				} else if objLabels, ok := s.Label["object"]; ok && len(objLabels) > 0 {
					typeName = objLabels[0]
				}
			}

			// Aggregate by type
			newMemory[typeName] += v
			if objCount > 0 {
				newObjects[typeName] += objCount
			}
		}
	}

	// Calculate memory growth
	type growthStat struct {
		Type           string
		OldValue       int64
		NewValue       int64
		Growth         int64
		GrowthPercent  float64
		OldCount       int64
		NewCount       int64
		CountGrowth    int64
		CountGrowthPct float64
	}

	growthStats := make([]growthStat, 0)

	for typeName, newVal := range newMemory {
		oldVal, exists := oldMemory[typeName]
		if !exists {
			oldVal = 0
		}

		growth := newVal - oldVal
		growthPct := 0.0
		if oldVal > 0 {
			growthPct = (float64(growth) / float64(oldVal)) * 100
		} else if growth > 0 {
			growthPct = 100.0 // New type, set growth rate to 100%
		}

		// Only focus on types with growth above the threshold
		if growthPct >= threshold*100 {
			newCount := newObjects[typeName]
			oldCount := oldObjects[typeName]
			countGrowth := newCount - oldCount
			countGrowthPct := 0.0
			if oldCount > 0 {
				countGrowthPct = (float64(countGrowth) / float64(oldCount)) * 100
			} else if countGrowth > 0 {
				countGrowthPct = 100.0
			}

			growthStats = append(growthStats, growthStat{
				Type:           typeName,
				OldValue:       oldVal,
				NewValue:       newVal,
				Growth:         growth,
				GrowthPercent:  growthPct,
				OldCount:       oldCount,
				NewCount:       newCount,
				CountGrowth:    countGrowth,
				CountGrowthPct: countGrowthPct,
			})
		}
	}

	// Sort by memory growth
	sort.Slice(growthStats, func(i, j int) bool {
		return growthStats[i].Growth > growthStats[j].Growth
	})

	// Format output
	var b strings.Builder
	b.WriteString("Memory Leak Detection Report\n")
	b.WriteString("==========================\n\n")

	if len(growthStats) == 0 {
		b.WriteString("No significant memory growth detected.\n")
		return b.String(), nil
	}

	b.WriteString(fmt.Sprintf("Found %d types with significant memory growth (threshold: %.1f%%)\n\n",
		len(growthStats), threshold*100))

	b.WriteString("Top Potential Memory Leaks:\n")
	b.WriteString("--------------------------------------------------\n")
	b.WriteString(fmt.Sprintf("%-20s %-15s %-15s %-15s %s\n",
		"Type", "Old Size", "New Size", "Growth", "Growth %"))
	b.WriteString("--------------------------------------------------\n")

	displayLimit := limit
	if displayLimit > len(growthStats) {
		displayLimit = len(growthStats)
	}

	for i := 0; i < displayLimit; i++ {
		stat := growthStats[i]
		b.WriteString(fmt.Sprintf("%-20s %-15s %-15s %-15s %.2f%%",
			stat.Type,
			FormatBytes(stat.OldValue),
			FormatBytes(stat.NewValue),
			FormatBytes(stat.Growth),
			stat.GrowthPercent))

		if stat.OldCount > 0 || stat.NewCount > 0 {
			b.WriteString(fmt.Sprintf(" (Objects: %d → %d, +%d, %.2f%%)",
				stat.OldCount, stat.NewCount, stat.CountGrowth, stat.CountGrowthPct))
		}

		b.WriteString("\n")
	}

	b.WriteString("\nRecommendations:\n")
	b.WriteString("1. Focus on types with both high absolute growth and high percentage growth\n")
	b.WriteString("2. Look for objects that grow in count but not significantly in size (may indicate collection leaks)\n")
	b.WriteString("3. Compare multiple snapshots over time to confirm consistent growth patterns\n")

	return b.String(), nil
}
