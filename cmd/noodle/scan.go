package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/TheGostsniperfr/Noodle/internal/inventory"
)

func scanCommand(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "inventory YAML output (stdout when empty)")
	namespace := fs.String("namespace", "default", "default namespace for Kubernetes declarations")
	var roots []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		roots = append(roots, fs.Arg(0))
		args = fs.Args()[1:]
	}
	report, err := inventory.Scan(ctx, roots, *namespace)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := inventory.Encode(&buf, report); err != nil {
		return err
	}
	if *out == "" {
		if _, err := stdout.Write(buf.Bytes()); err != nil {
			return err
		}
	} else {
		// An unsuccessful scan/write must not truncate the last usable inventory.
		file, err := os.CreateTemp(filepath.Dir(*out), ".noodle-inventory-*")
		if err != nil {
			return fmt.Errorf("scan output: %w", err)
		}
		name := file.Name()
		defer os.Remove(name)
		if _, err := file.Write(buf.Bytes()); err != nil {
			file.Close()
			return fmt.Errorf("scan output: %w", err)
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Rename(name, *out); err != nil {
			return fmt.Errorf("scan output: %w", err)
		}
	}
	var inputBytes int
	for _, file := range report.Files {
		inputBytes += int(file.Bytes)
	}
	fmt.Fprintf(stderr, "scan: %d roots, %d inputs, %d elements, %d connections, %d references, %d package manifests, %d unresolved; ~%s input tokens, ~%s inventory tokens\n",
		len(report.Roots), len(report.Files), len(report.Elements), len(report.Connections), len(report.References), len(report.Packages), len(report.Unresolved), kTokens(inputBytes), kTokens(buf.Len()))
	return nil
}
