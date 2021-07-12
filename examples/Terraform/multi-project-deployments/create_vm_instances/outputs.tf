output "folder_id" {
  description = "ID of the folder containing projects of interest."
  value       = local.folder_id
}

output "google_projects" {
  description = "List of projects inside a given folder"
  value       = local.projects
}

output "google_compute_instance_self_links" {
  description = "List of self-links for VM instances."
  value       = google_compute_instance.default
  sensitive   = true
}

output "google_compute_instance_ip" {
  description = "External IP addresses of VM instances."
  value       = google_compute_address.external_ip
}
