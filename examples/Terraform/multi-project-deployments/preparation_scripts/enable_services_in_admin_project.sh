#!/bin/bash
#
#  APIs to enable in the admin project
#

gcloud config set project "${TF_ADMIN_PROJECT}"

gcloud services enable cloudbilling.googleapis.com
gcloud services enable cloudresourcemanager.googleapis.com
gcloud services enable compute.googleapis.com
gcloud services enable iam.googleapis.com
gcloud services enable serviceusage.googleapis.com
gcloud services enable sourcerepo.googleapis.com
