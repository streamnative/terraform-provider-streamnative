---
page_title: "Provider: StreamNative"
subcategory: ""
description: |-
---

# StreamNative Provider

Simplify Apache Pulsar Terraform deployment with the StreamNative Terraform Provider. Manage Pulsar Instances, Pulsar Clusters, Service Accounts, and more in StreamNative.

Use the StreamNative provider to deploy and manage [StreamNative Cloud](https://console.streamnative.cloud) infrastructure. You must provide appropriate credentials to use the provider. The navigation menu provides details about the resources that you can interact with (_Resources_), and a guide (_Guides_) for how you can get started.

## Example Usage

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

[Add example]

## Enable StreamNative Cloud Access

[Document how to enable StreamNative Cloud Access]

## Provider Authentication

StreamNative Terraform provider allows authentication by using ...

## Helpful Links/Information

* [Report Bugs](https://github.com/streamnative/terraform-provider-streamnative/issues)