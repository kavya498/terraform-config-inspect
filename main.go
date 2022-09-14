package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/IBM-Cloud/terraform-config-inspect/tfconfig"
	flag "github.com/spf13/pflag"
)

var showJSON = flag.Bool("json", false, "produce JSON-formatted output")

var metadataJsonFile = flag.String("metadata", "", "Provider metadata json file path")
var showVariables = flag.Bool("filter-variables", false, "produce JSON-formatted output for variables")

// This function expects users to pass template path else it takes current path ./
func main() {
	flag.Parse()

	var dir string
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	} else {
		dir = "."
	}
	// If --metadata flag is provided, it parses through provider metdata file and extracts additional details of a given variable.
	// else it ll parse and fetch just the terraform template config.
	var module *tfconfig.Module
	if *metadataJsonFile != "" {
		var err tfconfig.Diagnostics
		module, err = tfconfig.LoadIBMModule(dir, *metadataJsonFile)
		if err != nil {
			err = append(err, tfconfig.Diagnostic{
				Severity: tfconfig.DiagError,
				Summary:  "loadErr",
				Detail:   fmt.Sprintf("%s", err),
			})
			log.Fatal(err)
		}
	} else {
		module, _ = tfconfig.LoadModule(dir)
	}

	if *showJSON {
		showModuleJSON(module, *showVariables)
	} else {
		showModuleMarkdown(module, *showVariables)
	}

	if module.Diagnostics.HasErrors() {
		os.Exit(1)
	}
}

func showModuleJSON(module *tfconfig.Module, variable bool) {

	if variable {
		metadataJson := tfconfig.Metadata{}
		variables := module.Variables
		for k, v := range variables {
			if v.Source != nil {
				v.Source = nil
			}
			// pos := tfconfig.SourcePos{}
			if v.Pos != nil {
				v.Pos = nil
			}
			variables[k] = v
		}
		metadataJson.Variables = variables
		outputs := module.Outputs
		for k, v := range outputs {
			if v.Source != nil {
				v.Source = nil
			}
			// pos := tfconfig.SourcePos{}
			if v.Pos != nil {
				v.Pos = nil
			}
			outputs[k] = v
		}
		metadataJson.Outputs = outputs
		j, err := json.MarshalIndent(metadataJson, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error producing JSON: %s\n", err)
			os.Exit(2)
		}
		os.Stdout.Write(j)
		os.Stdout.Write([]byte{'\n'})
	} else {
		j, err := json.MarshalIndent(module, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error producing JSON: %s\n", err)
			os.Exit(2)
		}
		os.Stdout.Write(j)
		os.Stdout.Write([]byte{'\n'})
	}

}

func showModuleMarkdown(module *tfconfig.Module, variable bool) {
	err := tfconfig.RenderMarkdown(os.Stdout, module, variable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error rendering template: %s\n", err)
		os.Exit(2)
	}
}
