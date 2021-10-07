output "folder_id" {
  description = "ID of the folder containing projects of interest."
  value       = local.folder_id
}

output "google_projects" {
  description = "List of projects inside a given folder"
  value       = local.projects
}

output "patch_deployments_self_links" {
  description = "List of self-links for OSConfig Patch Deployments."
  value       = google_os_config_patch_deployment.patch_deployments
}
