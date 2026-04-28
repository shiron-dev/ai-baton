resource "google_project_service" "required" {
  for_each = local.enable_project_services ? local.required_project_services : []

  project            = local.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_storage_bucket" "review_memory" {
  name                        = local.bucket_name
  location                    = local.bucket_location
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  depends_on = [
    google_project_service.required,
  ]

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      age                = 30
      num_newer_versions = 5
      with_state         = "ARCHIVED"
    }
    action {
      type = "Delete"
    }
  }
}

resource "google_service_account" "github_actions" {
  account_id   = local.service_account_id
  display_name = "review-harness GitHub Actions"

  depends_on = [
    google_project_service.required,
  ]
}

resource "google_storage_bucket_iam_member" "github_actions_object_user" {
  bucket = google_storage_bucket.review_memory.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_iam_workload_identity_pool" "github_actions" {
  workload_identity_pool_id = local.workload_identity_pool_id
  display_name              = "GitHub Actions"
  description               = "OIDC identities from GitHub Actions."

  depends_on = [
    google_project_service.required,
  ]
}

resource "google_iam_workload_identity_pool_provider" "github_actions" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_actions.workload_identity_pool_id
  workload_identity_pool_provider_id = local.workload_identity_provider_id
  display_name                       = "GitHub"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }

  attribute_condition = "assertion.repository == '${local.github_repository}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "github_actions_workload_identity_user" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_actions.name}/attribute.repository/${local.github_repository}"
}
