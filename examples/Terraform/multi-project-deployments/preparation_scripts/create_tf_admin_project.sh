#!/bin/bash

if [ -z "$TF_VAR_organization_id" ]
then
  echo "\$TF_VAR_organization_id is empty. You must set it first."
  exit 1
fi

if [ -z "$TF_VAR_billing_account" ]
then
  echo "\$TF_VAR_billing_account is empty. You must set it first."
  exit 1
fi

if [ -z "$TF_ADMIN_PROJECT" ]
then
  echo "\$TF_ADMIN_PROJECT is empty. You must set it first."
  exit 1
fi

if [ -z "$TF_ADMIN_USER" ]
then
  echo "\$TF_ADMIN_USER is empty. You must set it first."
  exit 1
fi


#
#  Create TF_ADMIN_PROJECT and set it up.
#

gcloud projects create ${TF_ADMIN_PROJECT} \
  --organization ${TF_VAR_organization_id} \
  --set-as-default

gcloud beta billing projects link ${TF_ADMIN_PROJECT} \
  --billing-account ${TF_VAR_billing_account}

gcloud config set project "${TF_ADMIN_PROJECT}"

export GOOGLE_PROJECT=${TF_ADMIN_PROJECT}


#
#   Enable services required for creating infrastructure
#

gcloud services enable cloudbilling.googleapis.com
gcloud services enable cloudresourcemanager.googleapis.com
gcloud services enable compute.googleapis.com
gcloud services enable iam.googleapis.com
gcloud services enable serviceusage.googleapis.com
gcloud services enable osconfig.googleapis.com


#
#   Grant TF_ADMIN_USER the necessary permissions
#

gcloud projects add-iam-policy-binding ${TF_ADMIN_PROJECT} \
  --member user:${TF_ADMIN_USER} \
  --role roles/viewer

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/resourcemanager.projectCreator

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/billing.user

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/resourcemanager.folderAdmin

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/compute.admin

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/serviceusage.serviceUsageConsumer

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/osconfig.guestPolicyAdmin

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/osconfig.osPolicyAssignmentAdmin

gcloud organizations add-iam-policy-binding ${TF_VAR_organization_id} \
  --member user:${TF_ADMIN_USER} \
  --role roles/osconfig.patchDeploymentAdmin
