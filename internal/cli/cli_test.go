package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func execute(args ...string) (out, errOut string, err error) {
	return executeIn(nil, args...)
}

func executeIn(in io.Reader, args ...string) (out, errOut string, err error) {
	return executeCtx(context.Background(), in, args...)
}

func executeCtx(ctx context.Context, in io.Reader, args ...string) (out, errOut string, err error) {
	var stdout, stderr bytes.Buffer
	root := New("27.0", "27A41", "2026-09-28", "3f2a9c1", &stdout, &stderr)
	root.SetIn(in)
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	err = root.ExecuteContext(ctx)
	return stdout.String(), stderr.String(), err
}

func mustExecute(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, err := execute(args...)
	require.NoError(t, err, "команда %q завершилась ошибкой", args)
	assert.Empty(t, errOut, "команда %q писала в stderr", args)
	return out
}
