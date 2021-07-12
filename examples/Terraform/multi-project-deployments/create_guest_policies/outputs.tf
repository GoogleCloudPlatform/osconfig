output "folder_id" {
  description = "ID of the folder containing projects of interest."
  value       = local.folder_id
}

output "google_projects" {
  description = "List of projects inside a given folder"
  value       = local.projects
}

output "guest_policies_self_links" {
  description = "List of self-links for OSConfig Guest Policies."
  value       = google_os_config_guest_policies.guest_policies
}
