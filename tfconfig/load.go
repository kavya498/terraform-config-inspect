package tfconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// LoadModule reads the directory at the given path and attempts to interpret
// it as a Terraform module.
func LoadModule(dir string) (*Module, Diagnostics) {
	return LoadModuleFromFilesystem(NewOsFs(), dir)
}

// LoadIBMModule takes template file directory and metadataPath as input and returns final module struct.
func LoadIBMModule(dir string, metadataPath string) (*Module, Diagnostics) {
	var metadata map[string]interface{}
	var err Diagnostics
	loadModule, LoadModuleFromFilesystemErr := LoadModuleFromFilesystem(NewOsFs(), dir)
	if LoadModuleFromFilesystemErr != nil {
		err = append(err, Diagnostic{
			Severity: DiagError,
			Summary:  "LoadModuleFromFilesystemErr",
			Detail:   fmt.Sprintf("%s", LoadModuleFromFilesystemErr),
		})
	}
	if metadataPath != "" {
		metadataBytes, metadataErr := ioutil.ReadFile(metadataPath)
		if metadataErr != nil {
			err = append(err, Diagnostic{
				Severity: DiagError,
				Summary:  "metadataErr",
				Detail:   fmt.Sprintf("Failed to read metadataPath file %s %s", metadataPath, metadataErr),
			})
		}

		unmarshalErr := json.Unmarshal([]byte(metadataBytes), &metadata)
		if unmarshalErr != nil {
			err = append(err, Diagnostic{
				Severity: DiagError,
				Summary:  "unmarshalErr",
				Detail:   fmt.Sprintf("Failed to unmarshal metadata json %s: %s", metadataPath, unmarshalErr),
			})
		}
	}
	// Once the template is loaded and the Module is extracted, find metadata for variables using Module struct and above metadata file.
	if loadModule.DataResources != nil {
		findMetadata("data", loadModule.DataResources, loadModule.Variables, metadata)
	}
	if loadModule.ManagedResources != nil {
		findMetadata("resource", loadModule.ManagedResources, loadModule.Variables, metadata)
	}
	if loadModule.ModuleCalls != nil && len(loadModule.ModuleCalls) != 0 {
		loadModuleErr := findModuleMetadata(dir, metadataPath, loadModule.ModuleCalls, loadModule.Variables, metadata)
		if loadModuleErr != nil {
			err = append(err, Diagnostic{
				Severity: DiagError,
				Summary:  "loadModuleErr",
				Detail:   fmt.Sprintf("Failed to load modules %s", loadModuleErr),
			})
		}
	}
	return loadModule, err
}

//findModuleMetadata:
// dir -->template file directory and metadataPath
// modules --> modules details from Module struct, variables --> variables from module struct and metadata json as inputs
// This function first checks for downloaded modules of terraform init under /.terraform/modules/  directory
// If the module name from modules struct matches any of the downloaded module, repeat the extraction LoadIBMModule
func findModuleMetadata(dir, metadataPath string, modules map[string]*ModuleCall, variables map[string]*Variable, metadata map[string]interface{}) Diagnostics {
	var err Diagnostics
	fileInfo, moduleDirErr := ioutil.ReadDir(dir + "/.terraform/modules/")
	if moduleDirErr != nil {
		err = append(err, Diagnostic{
			Severity: DiagError,
			Summary:  "moduleDirErr",
			Detail:   fmt.Sprintf("Failed to read init module directory of %s. Please run terraform init if it is not run earlier to load the modules.", dir+"/.terraform/modules/"),
		})
	}
	fileStruct := make(map[string]interface{})
	for _, f := range fileInfo {
		if f.IsDir() {
			fileStruct[f.Name()] = struct{}{}
		}
	}
	for _, module := range modules {
		if module.Attributes != nil {
			modulePath := dir + "/" + module.Source
			if _, ok := fileStruct[module.Name]; ok {
				modulePath = dir + "/.terraform/modules/" + module.Name
				modulePathSplit := strings.Split(module.Source, "//")
				if len(modulePathSplit) > 1 {
					modulePath = modulePath + "/" + modulePathSplit[1]
				}
			}
			// Load inner module
			loadedModulePath, LoadIBMModuleErr := LoadIBMModule(modulePath, metadataPath)
			if LoadIBMModuleErr != nil {
				err = append(err, Diagnostic{
					Severity: DiagError,
					Summary:  "LoadIBMModuleErr",
					Detail:   fmt.Sprintf("Error while loading child modules %s", LoadIBMModuleErr),
				})
			}

			if loadedModulePath.ManagedResources != nil {
				module.ManagedResources = loadedModulePath.ManagedResources
			}

			if loadedModulePath.DataResources != nil {
				module.DataResources = loadedModulePath.DataResources
			}
			// For attributes of modules if variable assigned to the attribute matches any of the Variables struct
			// and if moduleAttribute is present in inner module's variable reference,
			// assign all inner module's variable metadata to  modulevariable.
			for moduleAttribute, moduleVariableValue := range module.Attributes {
				if modulevariable, ok := variables[moduleVariableValue.(string)]; ok {
					source := "module." + module.Name
					if v, ok := loadedModulePath.Variables[moduleAttribute]; ok && len(v.Source) > 0 {
						source = source + "." + v.Source[len(v.Source)-1]
						modulevariable.Aliases = v.Aliases
						modulevariable.AllowedValues = v.AllowedValues
						modulevariable.CloudDataRange = v.CloudDataRange
						modulevariable.CloudDataType = v.CloudDataType
						modulevariable.Computed = v.Computed
						if modulevariable.Default == nil {
							modulevariable.Default = v.Default
						}
						modulevariable.Deprecated = v.Deprecated
						if modulevariable.Description == "" {
							modulevariable.Description = v.Description
						}
						modulevariable.Elem = v.Elem
						modulevariable.Hidden = v.Hidden
						modulevariable.Immutable = v.Immutable
						modulevariable.LinkStatus = v.LinkStatus
						modulevariable.Matches = v.Matches
						modulevariable.MaxItems = v.MaxItems
						modulevariable.MaxValue = v.MaxValue
						modulevariable.MaxValueLength = v.MaxValueLength
						modulevariable.MinItems = v.MinItems
						modulevariable.MinValue = v.MinValue
						modulevariable.MinValueLength = v.MinValueLength
						modulevariable.Optional = v.Optional
						if modulevariable.Required == nil {
							modulevariable.Required = v.Required
						}
						if modulevariable.Sensitive == nil {
							modulevariable.Sensitive = v.Sensitive
						}
					}
					modulevariable.Source = append(modulevariable.Source, source)
				}
			}
		}
	}
	return err
}

// findMetadata: This function is common for both resource and datasource.
// This takes moduleType --> data/resource,
// resources --> can be resource or datasource details from Module struct,
// variables --> variables from module struct and metadata json as inputs
// This checks if a variable reference is present in any of resource attributes.
// If found, it maps variable to resource/datasource, forms source and extracts provider metadata for that attribute using provider metadata json.

func findMetadata(moduleType string, resources map[string]*Resource, variables map[string]*Variable, metadata map[string]interface{}) {
	for _, resource := range resources {
		for resourceAttribute, resourceVariable := range resource.Attributes {
			if v, ok := variables[resourceVariable.(string)]; ok {
				source := resource.Type + "." + resource.Name + "." + resourceAttribute
				if moduleType == "data" {
					source = "data" + "." + source
					if d, ok := metadata["Datasources"]; ok {
						ExtractMetadata(v, d, resource.Type, resourceAttribute)
					}
				}
				if moduleType == "resource" {
					if r, ok := metadata["Resources"]; ok {
						ExtractMetadata(v, r, resource.Type, resourceAttribute)
					}
				}
				v.Source = append(v.Source, source)

			}
		}
	}
}

// ExtractMetadata: Takes v --> Variables,
// m --> resource or datasource metadata from metadata json file,
// moduleName -->  resource/datasource name
// moduleAttribute --> attribute of which metadata has to be extracted.
//This function check if moduleName has any metadata in m (metadata file), If present,
// It checks moduleAttribute is present in moduleName metadata, If Present, assign all the metadata to the variable v

func ExtractMetadata(v *Variable, m interface{}, moduleName, moduleAttribute string) {

	if ma, ok := m.(map[string]interface{})[moduleName]; ok {
		for _, argument := range ma.([]interface{}) {
			arg := argument.(map[string]interface{})
			if arg["name"] == moduleAttribute {
				if a, ok := arg["aliases"]; ok {
					v.Aliases = a.([]string)
				}
				if a, ok := arg["options"]; ok {
					v.AllowedValues = a.(string)
				}
				if a, ok := arg["cloud_data_type"]; ok {
					v.CloudDataType = a.(string)
				}
				if a, ok := arg["computed"]; ok {
					v.Computed = a.(bool)
				}
				if a, ok := arg["default"]; ok && v.Default == nil {
					v.Default = a
				}
				if a, ok := arg["description"]; ok && v.Description == "" {
					v.Description = a.(string)
				}
				if a, ok := arg["elem"]; ok {
					v.Elem = a
				}
				if a, ok := arg["hidden"]; ok {
					v.Hidden = a.(bool)
				}
				if a, ok := arg["immutable"]; ok {
					v.Immutable = a.(bool)
				}
				if a, ok := arg["link_status"]; ok {
					v.LinkStatus = a.(string)
				}
				if a, ok := arg["matches"]; ok {
					v.Matches = a.(string)
				}
				if a, ok := arg["max_items"]; ok {
					v.MaxItems = a.(int)
				}
				if a, ok := arg["max_value"]; ok {
					v.MaxValue = a.(string)
				}
				if a, ok := arg["min_items"]; ok {
					v.MinItems = a.(int)
				}
				if a, ok := arg["min_value"]; ok {
					v.MinValue = a.(string)
				}
				if a, ok := arg["min_length"]; ok {
					v.MinValueLength = a
				}
				if a, ok := arg["max_length"]; ok {
					v.MaxValueLength = a
				}
				if a, ok := arg["required"]; ok && v.Required == nil {
					required := a.(bool)
					v.Required = &required

				}
				if a, ok := arg["optional"]; ok {
					optional := a.(bool)
					v.Optional = &optional
				}
				if a, ok := arg["secure"]; ok && v.Sensitive == nil {
					v.Sensitive = a.(*bool)
				}
				if a, ok := arg["deprecated"]; ok {
					v.Deprecated = a.(string)
				}
				if a, ok := arg["cloud_data_range"]; ok {
					v.CloudDataRange = a.([]interface{})
				}
			}

		}
	}

}

// LoadModuleFromFilesystem reads the directory at the given path
// in the given FS and attempts to interpret it as a Terraform module
func LoadModuleFromFilesystem(fs FS, dir string) (*Module, Diagnostics) {
	// For broad compatibility here we actually have two separate loader
	// codepaths. The main one uses the new HCL parser and API and is intended
	// for configurations from Terraform 0.12 onwards (though will work for
	// many older configurations too), but we'll also fall back on one that
	// uses the _old_ HCL implementation so we can deal with some edge-cases
	// that are not valid in new HCL.

	module, diags := loadModule(fs, dir)
	if diags.HasErrors() {
		// Try using the legacy HCL parser and see if we fare better.
		legacyModule, legacyDiags := loadModuleLegacyHCL(fs, dir)
		if !legacyDiags.HasErrors() {
			legacyModule.init(legacyDiags)
			return legacyModule, legacyDiags
		}
	}

	module.init(diags)
	return module, diags
}

// IsModuleDir checks if the given path contains terraform configuration files.
// This allows the caller to decide how to handle directories that do not have tf files.
func IsModuleDir(dir string) bool {
	return IsModuleDirOnFilesystem(NewOsFs(), dir)
}

// IsModuleDirOnFilesystem checks if the given path in the given FS contains
// Terraform configuration files. This allows the caller to decide
// how to handle directories that do not have tf files.
func IsModuleDirOnFilesystem(fs FS, dir string) bool {
	primaryPaths, _ := dirFiles(fs, dir)
	if len(primaryPaths) == 0 {
		return false
	}
	return true
}

func (m *Module) init(diags Diagnostics) {
	// Fill in any additional provider requirements that are implied by
	// resource configurations, to avoid the caller from needing to apply
	// this logic itself. Implied requirements don't have version constraints,
	// but we'll make sure the requirement value is still non-nil in this
	// case so callers can easily recognize it.
	for _, r := range m.ManagedResources {
		if _, exists := m.RequiredProviders[r.Provider.Name]; !exists {
			m.RequiredProviders[r.Provider.Name] = &ProviderRequirement{}
		}
	}
	for _, r := range m.DataResources {
		if _, exists := m.RequiredProviders[r.Provider.Name]; !exists {
			m.RequiredProviders[r.Provider.Name] = &ProviderRequirement{}
		}
	}

	// We redundantly also reference the diagnostics from inside the module
	// object, primarily so that we can easily included in JSON-serialized
	// versions of the module object.
	m.Diagnostics = diags
}

func dirFiles(fs FS, dir string) (primary []string, diags hcl.Diagnostics) {
	infos, err := fs.ReadDir(dir)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Failed to read module directory",
			Detail:   fmt.Sprintf("Module directory %s does not exist or cannot be read.", dir),
		})
		return
	}

	var override []string
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

	// We are assuming that any _override files will be logically named,
	// and processing the files in alphabetical order. Primaries first, then overrides.
	primary = append(primary, override...)

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
