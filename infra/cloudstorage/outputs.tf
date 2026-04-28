output "cloudstorage_bucket" {
  description = "Cloud Storage bucket name for REVIEW_HARNESS_CLOUDSTORAGE_BUCKET."
  value       = google_storage_bucket.review_memory.name
}

output "github_actions_service_account_email" {
  description = "Service account email for GCP_SERVICE_ACCOUNT_EMAIL."
  value       = google_service_account.github_actions.email
}

output "workload_identity_provider" {
  description = "Provider resource name for GCP_WORKLOAD_IDENTITY_PROVIDER."
  value       = google_iam_workload_identity_pool_provider.github_actions.name
}
