package main

import (
	"cmp"
	"regexp"
)

var baseRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

func base(tags []string, v version) string {
	var best string
	var top version
	for _, tag := range tags {
		t, err := parse(tag)
		if err == nil && baseRe.MatchString(tag) && less(t, v) && (best == "" || less(top, t)) {
			best, top = tag, t
		}
	}
	return best
}

func less(a, b version) bool {
	return cmp.Or(cmp.Compare(a.major, b.major), cmp.Compare(a.minor, b.minor)) < 0
}
