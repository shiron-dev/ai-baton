locals {
  project_id      = "shiron-dev"
  region          = "asia-northeast1"
  bucket_name     = "shiron-dev-ai-baton-review-harness"
  bucket_location = "ASIA-NORTHEAST1"

  github_repository = "shiron-dev/ai-baton"

  enable_project_services       = true
  service_account_id            = "review-harness-actions"
  workload_identity_pool_id     = "github-actions"
  workload_identity_provider_id = "github"

  required_project_services = toset([
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "storage.googleapis.com",
  ])
}
