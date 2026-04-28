variable "project_id" {
  description = "GCP project ID where review-harness storage is created."
  type        = string
}

variable "region" {
  description = "Default provider region."
  type        = string
  default     = "asia-northeast1"
}

variable "bucket_name" {
  description = "Globally unique Cloud Storage bucket name for review-harness memory."
  type        = string
}

variable "bucket_location" {
  description = "Cloud Storage bucket location."
  type        = string
  default     = "ASIA-NORTHEAST1"
}

variable "github_repository" {
  description = "GitHub repository allowed to use Workload Identity Federation, in owner/name form."
  type        = string
}

variable "service_account_id" {
  description = "Service account ID used by GitHub Actions."
  type        = string
  default     = "review-harness-actions"
}

variable "workload_identity_pool_id" {
  description = "Workload Identity Pool ID for GitHub Actions."
  type        = string
  default     = "github-actions"
}

variable "workload_identity_provider_id" {
  description = "Workload Identity Provider ID for GitHub Actions."
  type        = string
  default     = "github"
}
