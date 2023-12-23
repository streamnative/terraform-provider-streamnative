# Terraform Provider for StreamNative cloud

# Installation

- From terraform registry(TODO)

  ```hcl
  terraform {
    required_providers {
      pulsar = {
        version = "0.1.0"
        source = "registry.terraform.io/streamnative/streamnative"
      }
    }
  }
  ```

- From source code

    - Clone this repository and cd into the directory
    - Run `make build`, it will generate a binary file named `terraform-provider-streamnative`
    - Copy this `terraform-provider-streamnative` binary file to your terraform plugin directory based on your OS:

| Operating System | User plugins directory                                                                        |
| ---------------- | --------------------------------------------------------------------------------------------- |
| Windows(amd64)   | %APPDATA%\terraform.d\plugins\registry.terraform.io\streamnative\streamnative\0.1.0\windows_amd64\  |
| Linux(amd64)     | ~/.terraform.d/plugins/registry.terraform.io/streamnative/streamnative/0.1.0/linux_amd64/           |
| MacOS(amd64)     | ~/.terraform.d/plugins/registry.terraform.io/streamnative/streamnative/0.1.0/darwin_amd64/          |

- Run `make build-dev`, it will build the binary and copy it to the plugin directory automatically.