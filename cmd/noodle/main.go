// noodle turns an architecture system (model, views and layouts, ADR-0007) into draw.io
// files in the house style, refusing to emit a diagram whose lines cross boxes or whose
// labels collide.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TheGostsniperfr/Noodle/internal/diagram"
	"github.com/TheGostsniperfr/Noodle/internal/house"
	"github.com/TheGostsniperfr/Noodle/internal/lint"
)

const slideMargin = 24.0

const usage = `usage: noodle render DIR [-view ID] [-format drawio|mermaid] [-o FILE] [-theme dark|light] [-icons DIR[,DIR]] [-slide]
        noodle scan ROOT… [-o inventory.yaml] [-namespace NS]
        noodle migrate SPEC.yaml DIR   (a v0 single file to a v1alpha1 system)
        noodle upgrade SOURCE_DIR NEW_DIR   (v1alpha1 to v1alpha2, evidence marked unknown)
       noodle discover ADAPTER [PATH…|-] [-o fragment.yaml]
       noodle -list-icons [-icons DIR[,DIR]]`

func main() {
	if len(os.Args) > 1 {
		commands := map[string]func([]string) error{"render": renderCommand, "migrate": migrateCommand, "upgrade": upgradeCommand, "scan": func(args []string) error {
			return scanCommand(context.Background(), args, os.Stdout, os.Stderr)
		}, "discover": func(args []string) error {
			reg, err := adapters()
			if err != nil {
				return err
			}
			return discoverCommand(context.Background(), reg, args, os.Stdin, os.Stdout, os.Stderr)
		}}
		if cmd, ok := commands[os.Args[1]]; ok {
			if err := cmd(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}
	icons := flag.String("icons", "", "comma-separated directories of extra <name>.svg icons, searched before the built-in ones")
	listIcons := flag.Bool("list-icons", false, "print the available icon names and exit")
	flag.Parse()

	if !*listIcons {
		if flag.NArg() == 1 && strings.HasSuffix(flag.Arg(0), ".yaml") {
			fmt.Fprintf(os.Stderr, "noodle: %s looks like a v0 single file, which is no longer rendered.\nConvert it once with: noodle migrate %s DIR, then: noodle render DIR\n", flag.Arg(0), flag.Arg(0))
		} else {
			fmt.Fprintln(os.Stderr, usage)
		}
		os.Exit(2)
	}
	set, err := newIconSet(splitList(*icons))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(set.names(), "\n"))
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lintAndRender(spec *diagram.Spec, out string, th *house.Theme, icons *iconSet, slide bool) error {
	errors := 0
	for _, f := range lint.Lint(spec) {
		level := "lint"
		if f.Warn {
			level = "warn"
		} else {
			errors++
		}
		fmt.Fprintf(os.Stderr, "%s: %-28s %s\n", level, f.Where, f.Msg)
	}
	if errors > 0 {
		return fmt.Errorf("%d lint finding(s)", errors)
	}
	if out == "" {
		return nil
	}
	if slide {
		spec.ForSlide(slideMargin)
	}
	r := &renderer{spec: spec, th: th, icons: icons, cache: map[string]string{}, ports: house.PortBadges(spec)}
	xml, err := r.render()
	if err != nil {
		return err
	}
	return os.WriteFile(out, []byte(xml), 0o644)
}
