#!/bin/bash
#
#  APIs to enable in the admin project
#

if [ -z "$TF_ADMIN_PROJECT" ]
then 
  echo "\$TF_ADMIN_PROJECT is empty. You must set it first."
  exit 1	
fi


gcloud config set project "${TF_ADMIN_PROJECT}"

gcloud services enable cloudbilling.googleapis.com
gcloud services enable cloudresourcemanager.googleapis.com
gcloud services enable compute.googleapis.com
gcloud services enable iam.googleapis.com
gcloud services enable serviceusage.googleapis.com
gcloud services enable sourcerepo.googleapis.com
