resource "google_folder" "tf_osconfig_folder" {
  display_name = var.folder_name
  parent       = "organizations/${var.organization_id}"
}

module "project-tf" {
  source = "terraform-google-modules/project-factory/google"

  count = 3

  name              = "tf-osconfig-test-${count.index}"
  random_project_id = "true"
  org_id            = var.organization_id
  billing_account   = var.billing_account
  folder_id         = google_folder.tf_osconfig_folder.id
}
