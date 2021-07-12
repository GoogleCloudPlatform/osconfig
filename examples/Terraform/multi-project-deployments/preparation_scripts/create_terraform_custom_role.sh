#!/bin/bash
#
#   Create IAM Custom role to use Terraform to create infrastructure
#   such as Folders, Projects, VM instances, networks and firewalls.
#

gcloud iam roles create TerraformDeployer5 \
--organization=${TF_VAR_organization_id} \
--file=../auth/TerraformDeployer.yaml

