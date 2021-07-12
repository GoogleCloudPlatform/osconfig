terraform {
  required_version = ">=0.13.0"
  required_providers {
    google  = ">= 3.43, <4.0"
    archive = "~> 1.0"
    random  = "~> 2.0"
    null    = "~> 2.1"
  }
  provider_meta "google" {
    module_name = "blueprints/terraform/terraform-google-vm:compute_instance/v6.1.0"
  }
}
