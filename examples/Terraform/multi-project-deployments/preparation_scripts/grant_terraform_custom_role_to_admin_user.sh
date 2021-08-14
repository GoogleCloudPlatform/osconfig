#!/bin/bash
#
#  Grant the Terraform IAM Custom role to the user who will be launching Terraform commands.
#

if [ -z "$TF_VAR_organization_id" ]
then 
  echo "\$TF_VAR_organization_id is empty. You must set it first."
  exit 1	
fi

if [ -z "$TF_ADMIN_USER_EMAIL" ]
then 
  echo "\$TF_ADMIN_USER_EMAIL is empty. You must set it first."
  exit 1	
fi

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
--member="user:${TF_ADMIN_USER_EMAIL}" \
--role="organizations/${TF_VAR_organization_id}/roles/TerraformDeployer6"

