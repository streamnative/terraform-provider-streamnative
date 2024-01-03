# Terraform Provider for StreamNative

Authored by [StreamNative](https://streamnative.io/), the StreamNative Terraform Provider is a plugin for Terraform that allows for the lifecycle management of StreamNative Cloud resources.

## Documentation

Full documentation is available on the [Terraform website](https://registry.terraform.io/providers/streamnative/streamnative/latest/docs).

## Contributing

Contributions are warmly welcomed and greatly appreciated! The project follows the typical GitHub pull request model. Please read the [contribution guidelines](CONTRIBUTING.md) for more details.

Before starting any work, please either comment on an existing issue, or file a new one.

## License

This library is licensed under the terms of the [Apache License 2.0](LICENSE) and may include packages written by third parties which carry their own copyright notices and license terms.

## About StreamNative

Founded in 2019 by the original creators of Apache Pulsar, [StreamNative](https://streamnative.io/) is one of the leading contributors to the open-source Apache Pulsar project. We have helped engineering teams worldwide make the move to Pulsar with [StreamNative Cloud](https://streamnative.io/product), a fully managed service to help teams accelerate time-to-production.

## FAQ

### Why don't you use this framework https://github.com/hashicorp/terraform-plugin-framework?
This project relies on the cloud-cli project, cloud-cli doesn't work with go 1.20 yet, I tried to use the old version in the project but failed, we should consider migrating to this framework in the future.

### Why don't you use the latest version https://github.com/hashicorp/terraform-plugin-sdk/tree/v2.31.0?
This project relies on the cloud-cli project, cloud-cli doesn't work with go 1.20 yet.
