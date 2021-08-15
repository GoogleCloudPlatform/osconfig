variable "organization_id" {
  description = "Cloud Organization where to create Projects."
  type        = string
}

variable "labels" {
  description = "Labels, provided as a map"
  type        = map(string)
}

variable "patch_deployment_execute_time" {
  description = "Time to wait before deploying the patch jobs."
  type        = string
}
