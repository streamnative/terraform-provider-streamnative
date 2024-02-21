# How to contribute

If you would like to contribute code to this project, fork the repository and send a pull request.

# Development Environment Setup

## Prerequisites

- [Terraform](https://www.terraform.io/downloads.html) 1.15.7 or later
- [Go](https://golang.org/doc/install) 1.19

## Build from source code 

- Clone this repository and cd into the directory
- Run `make build`, it will generate a binary file named `terraform-provider-streamnative`
- Copy this `terraform-provider-streamnative` binary file to your terraform plugin directory based on your OS:
  | Operating System | User plugins directory                                                                        |
  | ---------------- | --------------------------------------------------------------------------------------------- |
  | Windows(amd64)   | %APPDATA%\terraform.d\plugins\registry.terraform.io\streamnative\streamnative\0.1.0\windows_amd64\  |
  | Linux(amd64)     | ~/.terraform.d/plugins/registry.terraform.io/streamnative/streamnative/0.1.0/linux_amd64/           |
  | MacOS(amd64)     | ~/.terraform.d/plugins/registry.terraform.io/streamnative/streamnative/0.1.0/darwin_amd64/          |

- Run `make build-dev`, it will build the binary and copy it to the plugin directory automatically.

## OR

## Using .terraformrc

- Make sure GOBIN is set (if not set it to `/Users/<Username>/go/bin`)
- Create a file in `~` named `.terraformrc` 
- Add the following into the file 
```
provider_installation {

  dev_overrides {
      "terraform.local/local/streamnative" = "/Users/<Username>/go/bin" #Or your GOBIN if it's defined as a different path
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```
- Run `go install .` in the provider root
- Use the provider in terraform like so
```
terraform {
  required_providers {
    streamnative = {
      source = "terraform.local/local/streamnative"
    }
  }
}
```
- Run a terraform plan and terraform should use the newly built copy