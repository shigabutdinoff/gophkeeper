package main

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
)

const (
	maxMinor = 'Z' - 'A'
	maxBeta  = 'z' - 'a' + 1
)

var versionRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:-(beta|rc)(?:\.([1-9]\d*))?)?$`)

type version struct {
	major, minor, n int
	stage           string
}

func parse(s string) (version, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return version{}, fmt.Errorf("неверная версия %q", s)
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	n, err3 := strconv.Atoi(cmp.Or(m[5], "1"))
	if err1 != nil || err2 != nil || err3 != nil {
		return version{}, fmt.Errorf("неверная версия %q", s)
	}
	if minor > maxMinor {
		return version{}, fmt.Errorf("минор %d больше %d, для него нет буквы", minor, maxMinor)
	}
	if m[4] == "beta" && n > maxBeta {
		return version{}, fmt.Errorf("бета %d больше %d, для неё нет буквы", n, maxBeta)
	}
	return version{major: major, minor: minor, n: n, stage: m[4]}, nil
}

func format(v version, count int) string {
	letter := 'A' + rune(v.minor)
	if v.stage == "beta" {
		return fmt.Sprintf("%d%c5%03d%c", v.major, letter, count, 'a'+rune(v.n)-1)
	}
	return fmt.Sprintf("%d%c%d", v.major, letter, count)
}
