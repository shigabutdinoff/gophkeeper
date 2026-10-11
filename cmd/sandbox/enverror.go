package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

func envError(err error) error {
	agg, ok := errors.AsType[env.AggregateError](err)
	if !ok {
		return err
	}
	keys := make([]string, 0, len(agg.Errors))
	for _, e := range agg.Errors {
		if empty, isEmpty := errors.AsType[env.EmptyVarError](e); isEmpty {
			keys = append(keys, empty.Key)
		}
	}
	if bad, isBad := errors.AsType[env.ParseError](err); isBad && len(keys) == 0 {
		return bad.Err
	}
	return fmt.Errorf("не заданы %s", strings.Join(keys, ", "))
}
