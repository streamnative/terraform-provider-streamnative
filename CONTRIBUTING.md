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
