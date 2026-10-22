package main

import (
	"fmt"

	"github.com/TheGostsniperfr/Noodle/internal/model"
)

func upgradeCommand(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: noodle upgrade SOURCE_DIR NEW_DIR (v1alpha1 to v1alpha2)")
	}
	return model.UpgradeDirectory(args[0], args[1])
}
