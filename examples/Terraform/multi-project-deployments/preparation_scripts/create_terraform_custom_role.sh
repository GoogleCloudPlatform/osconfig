#!/bin/bash
#
#   Create IAM Custom role to use Terraform to create infrastructure
#   such as Folders, Projects, VM instances, networks and firewalls.
#

if [ -z "$TF_VAR_organization_id" ]
then 
  echo "\$TF_VAR_organization_id is empty. You must set it first."
  exit 1	
fi

gcloud iam roles create TerraformDeployer6 \
--organization=${TF_VAR_organization_id} \
--file=../auth/TerraformDeployer.yaml

