package targets

import "alikazai/ktree/internal/install"

var allTargets = []install.Target{
	Claude(),
	OpenCode(),
	Codex(),
	Grok(),
	Cursor(),
}

func All() []install.Target {
	cloned := make([]install.Target, len(allTargets))
	copy(cloned, allTargets)
	return cloned
}

func Lookup(key string) (install.Target, bool) {
	for _, target := range allTargets {
		if target.Key() == key {
			return target, true
		}
	}

	return nil, false
}
