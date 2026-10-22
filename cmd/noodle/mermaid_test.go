package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMermaidCommand(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "views"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "model.yaml"), []byte(`apiVersion: noodle/v1alpha2
kind: Model
elements: [{id: api, kind: backend, title: API, tech: Go}]
evidence: {api: {how: unknown, note: Test fixture has no source.}}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "views", "overview.yaml"), []byte(`apiVersion: noodle/v1alpha2
kind: View
type: topology
title: Architecture
`), 0o644))
	out := filepath.Join(t.TempDir(), "overview.mmd")
	require.NoError(t, renderCommand([]string{dir, "--format", "mermaid", "-o", out}))
	first, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(first), "flowchart LR")
	assert.Contains(t, string(first), "theme: dark")
	assert.Contains(t, string(first), "lineHops: false")
	assert.Contains(t, string(first), "API")
	require.NoError(t, renderCommand([]string{"--format", "mermaid", dir, "-o", out}))
	second, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
	require.NoError(t, renderCommand([]string{dir, "--format", "mermaid"}))
	assert.ErrorContains(t, renderCommand([]string{dir}), "check finding")
	assert.ErrorContains(t, renderCommand([]string{dir, "--format", "other"}), "unknown format")
	assert.ErrorContains(t, renderCommand([]string{dir, "--format", "mermaid", "--slide"}), "drawio options")
	assert.ErrorContains(t, renderCommand([]string{dir, "--format", "mermaid", "--view", "missing"}), "no view")
	require.NoError(t, renderCommand([]string{dir, "--format", "mermaid", "--theme", "light", "-o", out}))
	light, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(light), "theme: default")
}

func TestUpgradeCommand(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "upgraded")
	require.NoError(t, upgradeCommand([]string{"../../examples/cnp-runtime", dst}))
	assert.Error(t, upgradeCommand([]string{"../../examples/cnp-runtime", dst}))
	assert.ErrorContains(t, upgradeCommand(nil), "usage:")
}
