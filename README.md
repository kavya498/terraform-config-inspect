# terraform-config-inspect

This repository contains a helper library for extracting high-level metadata about Terraform modules from their source code and also provider schema metadata for respective variables defined for IBM-Cloud resources. It processes only a subset of the information Terraform itself would process, and in return it's able to be broadly compatible with modules written for many different versions of Terraform (>=1.0).

## Background

This tool has been enhanced to do the following:
* augment the `variable metadata` (in the Terraform module or template), with the metadata of the correspondng variables, extracted from the Terraform provider.
* override the `variable metadata` with the user-defined metadata of the corresponding variables in the imported modules

### Inputs

This tool uses the following inputs (optional)
* Terraform Provider metadata (eg. IBM Cloud Provider metadata) in json format.  
* Terraform Module metadata in json format 

> **Note:** 
> * The Terraform Provider for IBM Cloud (https://github.com/IBM-Cloud/terraform-provider-ibm) will include its Provider metadata, in the releases >=1.45.0. 
> * The Terraform Modules for IBM Cloud (https://github.com/terraform-ibm-modules) will also include the Module metadata file, in future releases.


### Variable metadata extracted by this tool

| Name | Type | Description |
|---|---|---|
| `name` | string | Variable name |
| `type` | string | Data type of the variable |
| `description` | string | Description of the variable |
| `default` | bool | Default value of the variable |
| `required` | bool | Whether the variable is required |
| `sensitive` | bool | Whether the variable contains credentials, secrets, or other sensitive values |
| `source` | string | Source identifier of the module in the form `<resource/data_source/module_name>.<resource/data_source/module_identifier>` |
|`pos`|object{filename:"path/to/file/name",line:line number}|position of the variable in the template|
| `aliases` | list(string) | The list of aliases for the variable name |
| `cloud_data_type` | string | The type of IBM Cloud data. Allowable values: `Region`, `ResourceInstance`, `CRN`, `Tags`, `ResourceGroup` |
|`link_status`|string|The status of the link|
| `immutable` | bool | If `true`, altering the value of the variable destroys and re-creates a resource (`ForceNew` behavior) |
| `hidden` | bool | If `true`, the variable is not displayed in the UI or CLI. |
| `options` | list(string) | Allowable values for the variable |
| `min_value` | string | Minimum value of a number variable. Validation is defined in the provider |
| `max_value` | string | Maximum value of number variable. Validation is defined in the provider |
| `min_value_length` | int | The minimum length of a string variable. Validation is defined in the provider. |
| `max_value_length` | int | The maximum length of a string variable. Validation is defined in the provider. |
| `matches` | string | The regular expression (regex) for the variable value. |
| `optional` | bool | Whether the variable is optional (`Optional` behavior) |
| `computed` | bool | Whether the variable is computed or derived (`Computed` behavior) |
| `elem` | provider schema struct | Child arguments of complex variable types |
| `max_items` | int | Maximum number of items for a `list` or `set` variable. Validation is defined in the provider schema. |
| `min_items` | int | Minimum number of items for a list or set variable. Validation is defined in the provider schema. |
| `deprecated` | bool | Whether the variable is deprecated in the provider schema. |
| `cloud_data_range` | string | The range of IBM Cloud data for the `CloudDataType`. For the `ResourceInstance` data type, the format is `["service:", ":"]`. |



---

## Install

The releases for `terraform-config-inspect` can be found here - https://github.com/hashicorp/terraform-config-inspect/releases

You can install `terraform-config-inspect` using the following commands:

```
$ go get github.com/ibm-cloud/terraform-config-inspect
```

You can also build and install the latest version of `terraform-config-inspect` :

1. Clone Repo
   ```
   git clone git@github.com:IBM-Cloud/terraform-config-inspect.git
   ```
2. Run `go build` on this repo.  
   > It generates binary in current working directory. 
   >
   > Add this binary to GOPATH to access from any location.

---

## Usage

The primary way to use this `terraform-config-inspect` CLI tool is as follows:

### Usage 1: Print output in console
  ```sh
  $ terraform-config-inspect path/to/module
  ```

   <details>
   <summary>Console output</summary>

    ```markdown
    # Module `path/to/module`

    Provider Requirements:
    * **null:** (any version)

    ## Input Variables
    * `a` (default `"a default"`)
    * `b` (required): The b variable

    ## Output Values
    * `a`
    * `b`: I am B

    ## Managed Resources
    * `null_resource.a` from `null`
    * `null_resource.b` from `null`
    ```
   </details>

### Usage 2: Print JSON output in console

  ```sh
  $ terraform-config-inspect path/to/module --json
  ```

  <details>
  <summary>JSON output</summary>

    ```json
    {
      "path": "path/to/module",
      "variables": {
        "A": {
          "name": "A",
          "default": "A default",
          "pos": {
            "filename": "path/to/module/basics.tf",
            "line": 1
          }
        },
        "B": {
          "name": "B",
          "description": "The B variable",
          "pos": {
            "filename": "path/to/module/basics.tf",
            "line": 5
          }
        }
      },
      "outputs": {
        "A": {
          "name": "A",
          "pos": {
            "filename": "path/to/module/basics.tf",
            "line": 9
          }
        },
        "B": {
          "name": "B",
          "description": "I am B",
          "pos": {
            "filename": "path/to/module/basics.tf",
            "line": 13
          }
        }
      },
      "required_providers": {
        "null": []
      },
      "managed_resources": {
        "null_resource.A": {
          "mode": "managed",
          "type": "null_resource",
          "name": "A",
          "provider": {
            "name": "null"
          },
          "pos": {
            "filename": "path/to/module/basics.tf",
            "line": 18
          }
        },
        "null_resource.B": {
          "mode": "managed",
          "type": "null_resource",
          "name": "B",
          "provider": {
            "name": "null"
          },
          "pos": {
            "filename": "path/to/module/basics.tf",
            "line": 19
          }
        }
      },
      "data_resources": {},
      "module_calls": {}
    }
    ```

  </details>


### Usage 3: Annotate with provider metadata 

  ```sh
  $ terraform-config-inspect path/to/module --json --metadata path/to/provider-metadata-file
  ```

Use the  `--metadata` flag to specify the location of the IBM Cloud provider metadata json file.

### Usage 4: Output variable metadata

  ```sh
  $ terraform-config-inspect path/to/module --json --filter-variables 
  ```

Use the `--filter-variables` flag include variables in the output metadata file

* This tool doesn't extract provider metadata of other cloud providers like AWS, Azure, GCP etc. while it doesn't fail when these providers are used. It gives high level template metadata as usual for non IBM-Cloud providers.

```sh
$ terraform-config-inspect --json path/to/module --metadata path/to/provider-metadata-file --filter-variables
```

  <details>
  <summary>Output variable metadata in JSON format</summary>

    ```json
    {
        "A": {
          "name": "A",
          "default": "A default",
          "min_length": 1,
          "max_length": 63,
          "matches": "^([a-z]|[a-z][-a-z0-9]*[a-z0-9])$",
          "optional": true,
          "computed": true
        },
        "B": {
          "name": "B",
          "description": "The B variable",
          "default": "bx2.4x16",
          "required": true,
          "immutable": true
        }
    }
    ```

  </details>

---

## Next steps
1. Extract template level variable validation.
2. Extract locals block metadata.