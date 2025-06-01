# Overview

<div class="warning">
This is a derivative of [nektos/act](https://github.com/nektos/act) between version v0.2.71 from January 2025 and v0.2.72 February 2025
</div>

- Support for macOS VMs using tart `-P tart://`
- `--use-new-action-cache` has been removed, the default clone mode of nektos/act has been removed
- CI tests are run in 6min compared to 17min on nektos/act
- Flags `--pull=false` and `--rebuild=false` are inverted to `--no-poll` and `--no-rebuild`

# Act User Guide

Please look at the [act user guide](https://actions-oss.github.io/act-docs/) for more documentation.

# Support

Need help? Ask on [Discussions](https://github.com/actions-oss/act-cli/discussions)!

# Contributing

Want to contribute to act? Awesome! Check out the [contributing guidelines](CONTRIBUTING.md) to get involved.

## Manually building from source

- Install Go tools 1.24+ - (<https://golang.org/doc/install>)
- Clone this repo `git clone git@github.com:actions-oss/act-cli.git`
- Run unit tests with `make test`
- Build and install: `make install`
