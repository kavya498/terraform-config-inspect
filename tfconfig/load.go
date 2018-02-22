package tfconfig

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl2/hcl"
)

// LoadModule reads the directory at the given path and attempts to interpret
// it as a Terraform module.
func LoadModule(dir string) (*Module, Diagnostics) {

	// For broad compatibility here we actually have two separate loader
	// codepaths. The main one uses the new HCL parser and API and is intended
	// for configurations from Terraform 0.12 onwards (though will work for
	// many older configurations too), but we'll also fall back on one that
	// uses the _old_ HCL implementation so we can deal with some edge-cases
	// that are not valid in new HCL.

	module, diags := loadModule(dir)
	if diags.HasErrors() {
		// Try using the legacy HCL parser and see if we fare better.
		legacyModule, legacyDiags := loadModuleLegacyHCL(dir)
		if !legacyDiags.HasErrors() {
			return legacyModule, legacyDiags
		}
	}
	return module, diags
}
func dirFiles(dir string) (primary, override []string, diags hcl.Diagnostics) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Failed to read module directory",
			Detail:   fmt.Sprintf("Module directory %s does not exist or cannot be read.", dir),
		})
		return
	}

	for _, info := range infos {
		if info.IsDir() {
			// We only care about files
			continue
		}

		name := info.Name()
		ext := fileExt(name)
		if ext == "" || isIgnoredFile(name) {
			continue
		}

		baseName := name[:len(name)-len(ext)] // strip extension
		isOverride := baseName == "override" || strings.HasSuffix(baseName, "_override")

		fullPath := filepath.Join(dir, name)
		if isOverride {
			override = append(override, fullPath)
		} else {
			primary = append(primary, fullPath)
		}
	}

	return
}

// fileExt returns the Terraform configuration extension of the given
// path, or a blank string if it is not a recognized extension.
func fileExt(path string) string {
	if strings.HasSuffix(path, ".tf") {
		return ".tf"
	} else if strings.HasSuffix(path, ".tf.json") {
		return ".tf.json"
	} else {
		return ""
	}
}

// isIgnoredFile returns true if the given filename (which must not have a
// directory path ahead of it) should be ignored as e.g. an editor swap file.
func isIgnoredFile(name string) bool {
	return strings.HasPrefix(name, ".") || // Unix-like hidden files
		strings.HasSuffix(name, "~") || // vim
		strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#") // emacs
}
