variable "organization_id" {
  description = "Cloud Organization where to create Projects."
  type        = string
}

variable "folder_name" {
  description = "New folder in which to create Projects."
  type        = string
}

variable "billing_account" {
  description = "Billing Account to which charge the Projects."
  type        = string
}
