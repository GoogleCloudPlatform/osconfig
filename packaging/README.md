## Scripts and resources for packaging OSConfig agent.

For more information on Daisy and how workflows work, refer to the
[Daisy documentation](https://github.com/GoogleCloudPlatform/compute-image-tools/tree/master/daisy).

# Local builds

```shell
# Builds Debian packages from development branch locally, placing results in
# /tmp/debpackage
./packaging/build_deb.sh
```

# Workflow invocation

```shell
# Builds all package types from the development branch of a forked repo,
# placing results in GCS bucket.
./daisy -project YOUR_PROJECT \
        -zone ZONE \
        -var:base_repo=MY_USERNAME \
        -var:pull_ref=MY_BRANCH \
        build_packages.wf.json
```

# Variables

All variables are optional.

*   `base_repo` Specify a different base for github repo (for example a fork of
    this repo). Default: `GoogleCloudPlatform`.
*   `repo` Specify a different github repo. Default: `osconfig`.
*   `pull_ref` Specify a git reference to check out. Default: `master`.
