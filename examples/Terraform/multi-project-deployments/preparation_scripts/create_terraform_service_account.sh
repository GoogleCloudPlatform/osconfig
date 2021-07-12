#!/bin/bash
#
#   Create a service account dedicated to managing infrastructure
#   via Terraform commands.
#
#


SERVICE_ACCOUNT_NAME="terraform-infra"

gcloud iam service-accounts create "${SERVICE_ACCOUNT_NAME}"


#
#  Grant the IAM Custom Role for Terraform to the service account
#
gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
--member="serviceAccount:${SERVICE_ACCOUNT_NAME}@${TF_ADMIN_PROJECT}.iam.gserviceaccount.com" \
--role=organizations/${TF_VAR_organization_id}/roles/TerraformDeployer5


#
#  Create and download keys from the service account
#
gcloud iam service-accounts keys create \
../auth/terraform_deployer.json \
--key-file-type=json \
--iam-account=${SERVICE_ACCOUNT_NAME}@${TF_ADMIN_PROJECT}.iam.gserviceaccount.com


export GOOGLE_APPLICATION_CREDENTIALS=../auth/terraform_deployer.json
