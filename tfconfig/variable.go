package tfconfig

// Variable represents a single variable from a Terraform module.
type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`

	// Default is an approximate representation of the default value in
	// the native Go type system. The conversion from the value given in
	// configuration may be slightly lossy. Only values that can be
	// serialized by json.Marshal will be included here.
	Default        interface{} `json:"default,omitempty"`
	Required       *bool       `json:"required,omitempty"`
	Sensitive      *bool       `json:"sensitive,omitempty"`
	Source         []string    `json:"source,omitempty"`
	Pos            *SourcePos  `json:"pos,omitempty"`
	Aliases        []string    `json:"aliases,omitempty" description:"The list of aliases for the variable name"`
	CloudDataType  string      `json:"cloud_data_type,omitempty" description:"Cloud data type of the variable. eg. resource_group_id, region, vpc_id."`
	LinkStatus     string      `json:"link_status,omitempty" description:"The status of the link"`
	Immutable      *bool       `json:"immutable,omitempty" description:"Is the variable readonly"`
	Hidden         *bool       `json:"hidden,omitempty" description:"If **true**, the variable is not displayed on UI or Command line."`
	AllowedValues  string      `json:"options,omitempty" description:"The Comma separated list of possible values for this variable.If type is **integer** or **date**, then the array of string is converted to array of integers or date during the runtime."`
	MinValue       string      `json:"min_value,omitempty" description:"The minimum value of the variable"`
	MaxValue       string      `json:"max_value,omitempty" description:"The maximum value of the variable"`
	MinValueLength interface{} `json:"min_length,omitempty" description:"The minimum length of the variable value. Applicable for the string type."`
	MaxValueLength interface{} `json:"max_length,omitempty" description:"The maximum length of the variable value. Applicable for the string type."`
	Matches        string      `json:"matches,omitempty" description:"The regex for the variable value."`

	//Values that are not present in schematics Variable Metadata
	Optional       *bool         `json:"optional,omitempty" description:""`
	Computed       *bool         `json:"computed,omitempty" description:""`
	Elem           interface{}   `json:"elem,omitempty" description:""`
	MaxItems       *int          `json:"max_items,omitempty" description:""`
	MinItems       *int          `json:"min_items,omitempty" description:""`
	Deprecated     string        `json:"deprecated,omitempty" description:""`
	CloudDataRange []interface{} `json:"cloud_data_range,omitempty" description:""`
}
