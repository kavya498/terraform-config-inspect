package tfconfig

// Output represents a single output from a Terraform module.
type Output struct {
	Name           string        `json:"name"`
	Description    string        `json:"description,omitempty"`
	Sensitive      bool          `json:"sensitive,omitempty"`
	Pos            *SourcePos    `json:"pos,omitempty"`
	Type           string        `json:"type,omitempty"`
	Source         []string      `json:"source,omitempty"`
	CloudDataType  string        `json:"cloud_data_type,omitempty" description:"Cloud data type of the variable. eg. resource_group_id, region, vpc_id."`
	CloudDataRange []interface{} `json:"cloud_data_range,omitempty" description:""`
}
