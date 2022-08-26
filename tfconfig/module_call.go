package tfconfig

// ModuleCall represents a "module" block within a module. That is, a
// declaration of a child module from inside its parent.
type ModuleCall struct {
	Name             string                 `json:"name"`
	Source           string                 `json:"source"`
	Version          string                 `json:"version,omitempty"`
	Attributes       map[string]interface{} `json:"attributes,omitempty"`
	ManagedResources map[string]*Resource   `json:"managed_resources"`
	DataResources    map[string]*Resource   `json:"data_resources"`

	Pos SourcePos `json:"pos"`
}
