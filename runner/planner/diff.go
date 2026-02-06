package planner

import "reflect"

func Diff(desired, current map[string]any) map[string]DiffEntry {
	result := map[string]DiffEntry{}
	seen := map[string]struct{}{}

	for k := range desired {
		seen[k] = struct{}{}
	}
	for k := range current {
		seen[k] = struct{}{}
	}

	for k := range seen {
		d := desired[k]
		c := current[k]
		if !reflect.DeepEqual(d, c) {
			result[k] = DiffEntry{Current: c, Desired: d}
		}
	}

	return result
}
