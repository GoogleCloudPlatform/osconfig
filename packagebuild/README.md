# Package build workflows

This directory contains the 'package builder', which is a set of [Daisy]
workflows and startup scripts. A package build workflow will accept a git
repository set up for package building, build it and produce a system package.

We use these packagebuild workflows from our Concourse pipelines and Prow jobs.

[Daisy]: https://github.com/GoogleCloudPlatform/compute-daisy

## Repo layout for package building

The package builder expects to check out a git repository that contains a
`packaging/` directory. This directory may contain one or more of:

* 1 or more RPM spec files.
* A `debian/` directory, following Debian packaging standards
* A `googet/` directory, containing [GooGet] specs and scripts

[GooGet]: https://github.com/google/googet

## Workflow invocation

Build workflows accept the following parameters:

* gcs\_path - where to uploaded the resulting package, e.g. gs://my-bucket/
* repo\_owner - the repo owner, for example 'GoogleCloudPlatform'
* repo\_name - the repo name, for example 'guest-agent'
* git\_ref - the git ref to check out, for example 'main'
* version - the version for the resulting package, for example '20221025.00'
* build\_dir - (optional) the subdirectory in the repo to `cd` into before
  starting the build

## Supported types

Each packagebuild workflow corresponds to a single package type. All builds
default to the x86\_64 architecture unless otherwise specified.

### Debian

Debian workflows launch an instance of the specified Debian release and produce
.deb packages.

Available workflows:

* build\_deb9.wf.json
* build\_deb10.wf.json
* build\_deb11.wf.json

An ARM64 workflow is also included:

* build\_deb11\_arm64.wf.json

### Enterprise Linux

EL (Enterprise Linux) workflows launch an instance of a predefined EL type and
produce .rpm packages. The workflows use the OS that was best suited at the time
of development.

Available workflows:

* build\_el6.wf.json - uses CentOS 6
* build\_el7.wf.json - uses CentOS 7
* build\_el8.wf.json - uses RHEL 8
* build\_el9.wf.json - uses CentOS Stream 9

ARM64 workflows are also included:

* build\_el8\_arm64.wf.json - uses the GCP optimized version of Rocky Linux 8
* build\_el9\_arm64.wf.json - uses RHEL 9

### GooGet

The `build_goo.wf.json` (GooGet) workflow launches a Debian 10 instance and
produces .goo packages.
