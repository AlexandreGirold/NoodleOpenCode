package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanCommandDeterministicOutputAndFailurePreservesInventory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "compose.yaml"), []byte("services: {api: {image: nginx:1.27, ports: [8080]}}\n"), 0o600))
	out := filepath.Join(root, "inventory.yaml")
	var stdout, stderr bytes.Buffer
	require.NoError(t, scanCommand(context.Background(), []string{root, "-o", out}, &stdout, &stderr))
	first, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "1 elements")
	require.NoError(t, scanCommand(context.Background(), []string{"-o", out, root}, &stdout, &stderr))
	second, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
	err = scanCommand(context.Background(), []string{filepath.Join(root, "missing"), "-o", out}, &stdout, &stderr)
	require.Error(t, err)
	kept, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, first, kept)
	stdout.Reset()
	require.NoError(t, scanCommand(context.Background(), []string{root}, &stdout, &stderr))
	assert.Equal(t, string(first), stdout.String())
	assert.Error(t, scanCommand(context.Background(), []string{"--not-a-flag"}, &stdout, &stderr))
}
