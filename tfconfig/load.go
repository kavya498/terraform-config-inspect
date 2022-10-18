package tfconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// LoadModule reads the directory at the given path and attempts to interpret
// it as a Terraform module.
func LoadModule(dir string) (*Module, Diagnostics) {
	return LoadModuleFromFilesystem(NewOsFs(), dir)
}

// LoadIBMModule takes template file directory and metadataPath as input and returns final module struct.
func CheckForInitDirectoryAndLoadIBMModule(dir string, metadataPath string) (*Module, Diagnostics) {
	var err Diagnostics
	fileStruct := make(map[string]interface{})
	// Check for init directory ./terraform and return error if it is not present
	_, initDirErr := ioutil.ReadDir(dir + "/.terraform/")
	if initDirErr != nil {
		err = append(err, Diagnostic{
			Severity: DiagError,
			Summary:  "initDirErr",
			Detail:   fmt.Sprintf("Failed to read init module directory of %s. Please run terraform init if it is not run earlier to load the modules: %s", dir+"/.terraform/", initDirErr),
		})
		return nil, err
	}

	// Check for modules directory ./terraform/modules and will not return error assuming that no modules are present.
	// Store directories inside the modules folders as file struct.
	moduleFilesInfo, moduleDirErr := ioutil.ReadDir(dir + "/.terraform/modules/")
	if moduleDirErr == nil {
		for _, f := range moduleFilesInfo {
			if f.IsDir() {
				fileStruct[f.Name()] = struct{}{}
			}
		}
	} else {
		log.Printf("No modules downloaded for %s", dir)
	}
	// LoadIBMModule to extract metadata
	loadedModule, loadedModuleErr := LoadIBMModule(dir, metadataPath, fileStruct)
	if loadedModuleErr != nil {
		err = append(err, Diagnostic{
			Severity: DiagError,
			Summary:  "loadedModuleErr",
			Detail:   fmt.Sprintf("Failed to LoadIBMModule for %s", loadedModuleErr),
		})
		return nil, err
	}
	return loadedModule, nil
}

// LoadIBMModule takes template file directory and metadataPath as input and returns final module struct.
func LoadIBMModule(dir string, metadataPath string, fileStruct map[string]interface{}) (*Module, Diagnostics) {
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
		findVariableMetadataFromResourceOrDatasource("data", loadModule.DataResources, loadModule.Variables, metadata)
	}
	if loadModule.ManagedResources != nil {
		findVariableMetadataFromResourceOrDatasource("resource", loadModule.ManagedResources, loadModule.Variables, metadata)
	}
	if loadModule.ModuleCalls != nil && len(loadModule.ModuleCalls) != 0 {
		loadModuleErr := findVariableMetadataFromModule(dir, metadataPath, fileStruct, loadModule.ModuleCalls, loadModule.Variables, metadata)
		if loadModuleErr != nil {
			err = append(err, Diagnostic{
				Severity: DiagError,
				Summary:  "loadModuleErr",
				Detail:   fmt.Sprintf("Failed to load modules %s", loadModuleErr),
			})
		}
	}
	if loadModule.Outputs != nil {
		findOutputMetadataFromResourceOrDatasource(loadModule.Outputs, metadata)
	}
	return loadModule, err
}

// SortedKeysOfMap
// when there is a range on map, keys are always picked in random.
// This function sorts keys of map and returns sorted list of keys
func SortedKeysOfMap(mapper interface{}) []string {
	keyValueMap := reflect.ValueOf(mapper).MapKeys()
	keyList := make([]string, len(keyValueMap))
	for k, v := range keyValueMap {
		keyList[k] = v.Interface().(string)
	}
	sort.Strings(keyList)
	return keyList
}

// findSubModuleSourcePath finds subfolder in a given source for all below scenarios
/*

	./consul
	"hashicorp/consul/aws"
	"app.terraform.io/example-corp/k8s-cluster/azurerm" - other registries
	"github.com/hashicorp/example"
	"git@github.com:hashicorp/example.git"
	"bitbucket.org/hashicorp/terraform-consul-aws"
	"git::https://example.com/vpc.git"
	"git::ssh://username@example.com/storage.git"
	"git::https://example.com/vpc.git?ref=v1.2.0"
	"git::https://example.com/storage.git?ref=51d462976d84fdea54b47d80dcabbf680badcdb8"
	"git::username@example.com:storage.git"
	"hg::http://example.com/vpc.hg"
	"hg::http://example.com/vpc.hg?ref=v1.2.0"
	"https://example.com/vpc-module.zip"
	"https://example.com/vpc-module?archive=zip"
	"s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip"
	"gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip"
	with subfolder:
	hashicorp/consul/aws//modules/consul-cluster
	git::https://example.com/network.git//modules/vpc
	https://example.com/network-module.zip//modules/vpc
	s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/network.zip//modules/vpc
*/

func findSubModuleSourcePath(source string) string {
	if strings.Contains(source, "?ref=") {
		source = strings.Split(source, "?ref=")[0]
	}
	if strings.Contains(source, "?archive=") {
		source = strings.Split(source, "?archive=")[0]
	}
	if strings.Contains(source, "https://") {
		source = strings.Split(source, "https://")[1]
	}
	if strings.Contains(source, "ssh://") {
		source = strings.Split(source, "ssh://")[1]
	}
	if strings.Contains(source, "http://") {
		source = strings.Split(source, "http://")[1]
	}
	if strings.Contains(source, "//") {
		subfolderPath := strings.Split(source, "//")
		return subfolderPath[1]
	}
	return ""
}

//findVariableMetadataFromModule:
// dir -->template file directory and metadataPath
// modules --> modules details from Module struct, variables --> variables from module struct and metadata json as inputs
// This function first checks for downloaded modules of terraform init under /.terraform/modules/  directory
// If the module name from modules struct matches any of the downloaded module, repeat the extraction LoadIBMModule
func findVariableMetadataFromModule(dir, metadataPath string, fileStruct map[string]interface{}, modules map[string]*ModuleCall, variables map[string]*Variable, metadata map[string]interface{}) Diagnostics {
	var err Diagnostics
	parentModuleName := ""
	if strings.Contains(dir, "/.terraform/modules/") && len(strings.Split(dir, "/.terraform/modules/")) > 1 {
		parentModuleName = fmt.Sprintf("%s.", strings.Split(dir, "/.terraform/modules/")[1])
	}
	// since range on map picks keys in random order,
	// we sort keys using SortedKeysOfMap and range on the the keys
	for _, mod := range SortedKeysOfMap(modules) {
		module := modules[mod]
		if module.Attributes != nil {
			var modulePath string
			if strings.HasPrefix(module.Source, "/") || strings.HasPrefix(module.Source, "./") || strings.HasPrefix(module.Source, "../") {
				modulePath = dir + "/" + module.Source
			} else if _, ok := fileStruct[parentModuleName+module.Name]; ok {
				// removing parent folder for child modules as child modules download under parent.child folder
				if strings.Contains(dir, "/.terraform/modules/") {
					rootDir := strings.Split(dir, "/.terraform/modules/")[0]
					modulePath = rootDir + "/.terraform/modules/" + parentModuleName + module.Name
				} else {
					modulePath = dir + "/.terraform/modules/" + parentModuleName + module.Name
				}

				subModuleDirectorySource := findSubModuleSourcePath(module.Source)
				if subModuleDirectorySource != "" {
					modulePath = modulePath + "/" + subModuleDirectorySource
				}
			} else {
				err = append(err, Diagnostic{
					Severity: DiagError,
					Summary:  "module path error",
					Detail:   fmt.Sprintf("module source %s is either incorrect or not supported by this tool", module.Source),
				})
				return err
			}
			log.Printf("[INFO] Loading module '%s' from the path '%s'", module.Name, modulePath)
			// Load inner module
			loadedModulePath, LoadIBMModuleErr := LoadIBMModule(modulePath, metadataPath, fileStruct)
			if LoadIBMModuleErr != nil {
				err = append(err, Diagnostic{
					Severity: DiagError,
					Summary:  "LoadIBMModuleErr",
					Detail:   fmt.Sprintf("Error while loading child modules %s", LoadIBMModuleErr),
				})
				return err
			}

			if loadedModulePath.ManagedResources != nil {
				module.ManagedResources = loadedModulePath.ManagedResources
			}

			if loadedModulePath.DataResources != nil {
				module.DataResources = loadedModulePath.DataResources
			}
			if loadedModulePath.Outputs != nil {
				module.Outputs = loadedModulePath.Outputs
			}
			// For attributes of modules if variable assigned to the attribute matches any of the Variables struct
			// and if moduleAttribute is present in inner module's variable reference,
			// assign all inner module's variable metadata to  modulevariable.
			for moduleAttribute, moduleVariableValue := range module.Attributes {
				if modulevariable, ok := variables[moduleVariableValue.(string)]; ok {
					source := "module." + module.Name
					if v, ok := loadedModulePath.Variables[moduleAttribute]; ok && len(v.Source) > 0 && (modulevariable.Type == "string" || modulevariable.Type == "number" || modulevariable.Type == "bool" || modulevariable.Type == "list(string)" || modulevariable.Type == "set(string)" || modulevariable.Type == "map") {

						if modulevariable.Aliases == nil {
							modulevariable.Aliases = v.Aliases
						}
						if modulevariable.AllowedValues == "" {
							modulevariable.AllowedValues = v.AllowedValues
						}
						if len(modulevariable.CloudDataRange) == 0 {
							modulevariable.CloudDataRange = v.CloudDataRange
						}
						if modulevariable.CloudDataType == "" {
							modulevariable.CloudDataType = v.CloudDataType
						}
						if modulevariable.Computed == nil {
							modulevariable.Computed = v.Computed
						}
						if modulevariable.Default == nil {
							modulevariable.Default = v.Default
						}
						if modulevariable.Deprecated == "" {
							modulevariable.Deprecated = v.Deprecated
						}
						if modulevariable.Description == "" {
							modulevariable.Description = v.Description
						}
						if modulevariable.Elem == nil {
							modulevariable.Elem = v.Elem
						}
						if modulevariable.Hidden == nil {
							modulevariable.Hidden = v.Hidden
						}
						if modulevariable.Immutable == nil {
							modulevariable.Immutable = v.Immutable
						}
						if modulevariable.LinkStatus == "" {
							modulevariable.LinkStatus = v.LinkStatus
						}
						if modulevariable.MaxItems == nil {
							modulevariable.MaxItems = v.MaxItems
						}
						if modulevariable.MaxValue == "" {
							modulevariable.MaxValue = v.MaxValue
						}
						if modulevariable.MaxValueLength == nil {
							modulevariable.MaxValueLength = v.MaxValueLength
						}
						if modulevariable.MinValueLength == nil {
							modulevariable.MinValueLength = v.MinValueLength
						}
						if modulevariable.Matches == "" {
							modulevariable.Matches = v.Matches
						}
						if modulevariable.MinItems == nil {
							modulevariable.MinItems = v.MinItems
						}
						if modulevariable.MinValue == "" {
							modulevariable.MinValue = v.MinValue
						}
						if modulevariable.Optional == nil {
							modulevariable.Optional = v.Optional
						}
						if modulevariable.Required == nil {
							modulevariable.Required = v.Required
						}
						if modulevariable.Sensitive == nil {
							modulevariable.Sensitive = v.Sensitive
						}
						for _, s := range v.Source {
							modulevariable.Source = append(modulevariable.Source, source+"."+s)
						}
					} else {
						modulevariable.Source = append(modulevariable.Source, source)
					}
					sort.Strings(modulevariable.Source)
				}
			}
		}
	}
	return err
}

// findVariableMetadataFromResourceOrDatasource: This function is common for both resource and datasource.
// This takes moduleType --> data/resource,
// resources --> can be resource or datasource details from Module struct,
// variables --> variables from module struct and metadata json as inputs
// This checks if a variable reference is present in any of resource attributes.
// If found, it maps variable to resource/datasource, forms source and extracts provider metadata for that attribute using provider metadata json.

func findOutputMetadataFromResourceOrDatasource(outputs map[string]*Output, metadata map[string]interface{}) {
	for _, o := range SortedKeysOfMap(outputs) {
		output := outputs[o]
		if output.Value != "" {
			splitOutput := strings.Split(output.Value, ".")
			if len(splitOutput) >= 4 {
				moduleType := splitOutput[len(splitOutput)-4]
				resourceOrDatasourceName := splitOutput[len(splitOutput)-3]
				resourceOrDatasourceAttribute := splitOutput[len(splitOutput)-1]
				if moduleType == "data" {
					if d, ok := metadata["Datasources"]; ok {
						ExtractOutputMetadata(output, d, resourceOrDatasourceName, resourceOrDatasourceAttribute)
					}
				}
				if moduleType != "data" && strings.HasPrefix(resourceOrDatasourceName, "ibm_") {
					if r, ok := metadata["Resources"]; ok {
						ExtractOutputMetadata(output, r, resourceOrDatasourceName, resourceOrDatasourceAttribute)
					}
				}
			}
		}
	}
}

// Name           string        `json:"name"`
// 	Description    string        `json:"description,omitempty"`
// 	Value          string        `json:"value,omitempty"`
// 	Sensitive      bool          `json:"sensitive,omitempty"`
// 	Pos            *SourcePos    `json:"pos,omitempty"`
// 	Type           string        `json:"type,omitempty"`
// 	Source         []string      `json:"source,omitempty"`
// 	CloudDataType  string        `json:"cloud_data_type,omitempty" description:"Cloud data type of the variable. eg. resource_group_id, region, vpc_id."`
// 	CloudDataRange []interface{} `json:"cloud_data_range,omitempty" description:""`
func ExtractOutputMetadata(o *Output, m interface{}, moduleName, moduleAttribute string) {

	if ma, ok := m.(map[string]interface{})[moduleName]; ok {
		for _, argument := range ma.([]interface{}) {
			arg := argument.(map[string]interface{})
			if arg["name"] == moduleAttribute {
				// if a, ok := arg["aliases"]; ok && o.Aliases == nil {
				// 	o.Aliases = a.([]string)
				// }
				// if a, ok := arg["options"]; ok && o.AllowedValues == "" {
				// 	o.AllowedValues = a.(string)
				// }
				if a, ok := arg["cloud_data_type"]; ok && o.CloudDataType == "" {
					o.CloudDataType = a.(string)
				}
				// if a, ok := arg["computed"]; ok && o.Computed == nil {
				// 	computed := a.(bool)
				// 	o.Computed = &computed
				// }
				// if a, ok := arg["default"]; ok && o.Default == nil {
				// 	o.Default = a
				// }
				if a, ok := arg["description"]; ok && o.Description == "" {
					o.Description = a.(string)
				}
				// if a, ok := arg["elem"]; ok && o.Elem == nil {
				// 	o.Elem = a
				// }
				// if a, ok := arg["hidden"]; ok && o.Hidden == nil {
				// 	hidden := a.(bool)
				// 	o.Hidden = &hidden
				// }
				// if a, ok := arg["immutable"]; ok && o.Immutable == nil {
				// 	immutable := a.(bool)
				// 	o.Immutable = &immutable
				// }
				// if a, ok := arg["link_status"]; ok && o.LinkStatus == "" {
				// 	o.LinkStatus = a.(string)
				// }
				// if a, ok := arg["matches"]; ok && o.Matches == "" {
				// 	o.Matches = a.(string)
				// }
				// if a, ok := arg["max_items"]; ok && o.MaxItems == nil {
				// 	maxItems := a.(int)
				// 	o.MaxItems = &maxItems
				// }
				// if a, ok := arg["max_value"]; ok && o.MaxValue == "" {
				// 	o.MaxValue = a.(string)
				// }
				// if a, ok := arg["min_items"]; ok && o.MinItems == nil {
				// 	minItems := a.(int)
				// 	o.MinItems = &minItems
				// }
				// if a, ok := arg["min_value"]; ok && o.MinValue == "" {
				// 	o.MinValue = a.(string)
				// }
				// if a, ok := arg["min_length"]; ok && o.MinValueLength == nil {
				// 	o.MinValueLength = a
				// }
				// if a, ok := arg["max_length"]; ok && o.MaxValueLength == nil {
				// 	o.MaxValueLength = a
				// }
				// if a, ok := arg["required"]; ok && o.Required == nil {
				// 	required := a.(bool)
				// 	o.Required = &required
				// }
				// if a, ok := arg["optional"]; ok && o.Optional == nil && (o.Required != nil && !*o.Required) {
				// 	optional := a.(bool)
				// 	o.Optional = &optional
				// }
				// if a, ok := arg["secure"]; ok && o.Sensitive == nil {
				// 	o.Sensitive = a.(*bool)
				// }
				// if a, ok := arg["deprecated"]; ok && o.Deprecated == "" {
				// 	o.Deprecated = a.(string)
				// }
				if a, ok := arg["cloud_data_range"]; ok && len(o.CloudDataRange) == 0 {
					o.CloudDataRange = a.([]interface{})
				}
			}

		}
	}

}

func findVariableMetadataFromResourceOrDatasource(moduleType string, resources map[string]*Resource, variables map[string]*Variable, metadata map[string]interface{}) {
	// since range on map picks keys in random order,
	// we sort keys using SortedKeysOfMap and range on the the keys
	for _, v := range SortedKeysOfMap(resources) {
		resource := resources[v]
		for resourceAttribute, resourceVariable := range resource.Attributes {
			if v, ok := variables[resourceVariable.(string)]; ok {
				source := resource.Type + "." + resource.Name + "." + resourceAttribute
				if moduleType == "data" {
					source = "data" + "." + source
					if d, ok := metadata["Datasources"]; ok {
						ExtractVariableMetadata(v, d, resource.Type, resourceAttribute)
					}
				}
				if moduleType == "resource" {
					if r, ok := metadata["Resources"]; ok {
						ExtractVariableMetadata(v, r, resource.Type, resourceAttribute)
					}
				}
				v.Source = append(v.Source, source)
				sort.Strings(v.Source)

			}
		}
	}
}

// ExtractVariableMetadata: Takes v --> Variables,
// m --> resource or datasource metadata from metadata json file,
// moduleName -->  resource/datasource name
// moduleAttribute --> attribute of which metadata has to be extracted.
//This function check if moduleName has any metadata in m (metadata file), If present,
// It checks moduleAttribute is present in moduleName metadata, If Present, assign all the metadata to the variable v

func ExtractVariableMetadata(v *Variable, m interface{}, moduleName, moduleAttribute string) {

	if ma, ok := m.(map[string]interface{})[moduleName]; ok {
		for _, argument := range ma.([]interface{}) {
			arg := argument.(map[string]interface{})
			if arg["name"] == moduleAttribute && (v.Type == "string" || v.Type == "number" || v.Type == "bool" || v.Type == "list(string)" || v.Type == "set(string)" || v.Type == "map") {

				if a, ok := arg["aliases"]; ok && v.Aliases == nil {
					v.Aliases = a.([]string)
				}
				if a, ok := arg["options"]; ok && v.AllowedValues == "" {
					v.AllowedValues = a.(string)
				}
				if a, ok := arg["cloud_data_type"]; ok && v.CloudDataType == "" {
					v.CloudDataType = a.(string)
				}
				if a, ok := arg["computed"]; ok && v.Computed == nil {
					computed := a.(bool)
					v.Computed = &computed
				}
				if a, ok := arg["default"]; ok && v.Default == nil {
					v.Default = a
				}
				if a, ok := arg["description"]; ok && v.Description == "" {
					v.Description = a.(string)
				}
				if a, ok := arg["elem"]; ok && v.Elem == nil {
					v.Elem = a
				}
				if a, ok := arg["hidden"]; ok && v.Hidden == nil {
					hidden := a.(bool)
					v.Hidden = &hidden
				}
				if a, ok := arg["immutable"]; ok && v.Immutable == nil {
					immutable := a.(bool)
					v.Immutable = &immutable
				}
				if a, ok := arg["link_status"]; ok && v.LinkStatus == "" {
					v.LinkStatus = a.(string)
				}
				if a, ok := arg["matches"]; ok && v.Matches == "" {
					v.Matches = a.(string)
				}
				if a, ok := arg["max_items"]; ok && v.MaxItems == nil {
					maxItems := a.(int)
					v.MaxItems = &maxItems
				}
				if a, ok := arg["max_value"]; ok && v.MaxValue == "" {
					v.MaxValue = a.(string)
				}
				if a, ok := arg["min_items"]; ok && v.MinItems == nil {
					minItems := a.(int)
					v.MinItems = &minItems
				}
				if a, ok := arg["min_value"]; ok && v.MinValue == "" {
					v.MinValue = a.(string)
				}
				if a, ok := arg["min_length"]; ok && v.MinValueLength == nil {
					v.MinValueLength = a
				}
				if a, ok := arg["max_length"]; ok && v.MaxValueLength == nil {
					v.MaxValueLength = a
				}
				if a, ok := arg["required"]; ok && v.Required == nil {
					required := a.(bool)
					v.Required = &required
				}
				if a, ok := arg["optional"]; ok && v.Optional == nil && (v.Required != nil && !*v.Required) {
					optional := a.(bool)
					v.Optional = &optional
				}
				if a, ok := arg["secure"]; ok && v.Sensitive == nil {
					v.Sensitive = a.(*bool)
				}
				if a, ok := arg["deprecated"]; ok && v.Deprecated == "" {
					v.Deprecated = a.(string)
				}
				if a, ok := arg["cloud_data_range"]; ok && len(v.CloudDataRange) == 0 {
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
