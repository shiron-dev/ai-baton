# review-harness Cloud Storage

Terraform for the review-harness persistent memory store on Google Cloud Storage and GitHub Actions OIDC authentication.

This creates:

- a Cloud Storage bucket for the SQLite memory DB
- a GitHub Actions service account
- a Workload Identity Pool / Provider trusting GitHub's OIDC issuer
- `roles/iam.workloadIdentityUser` for the configured GitHub repository
- `roles/storage.objectUser` on the memory bucket
- required GCP APIs: IAM, IAM Credentials, Security Token Service, and Cloud Storage

## Apply

Edit [`locals.tf`](./locals.tf) when the project, bucket name, or trusted repository changes.

```bash
cd infra/cloudstorage
terraform init
terraform apply
```

The Terraform state is stored in Cloud Storage:

```hcl
bucket = "shiron-dev-terraform"
prefix = "ai-baton"
```

## GitHub Variables / Secrets

Set these repository variables from the Terraform outputs. They are identifiers, not secret material, and keeping them in variables prevents `google-github-actions/auth` from receiving empty inputs on PR events where secrets are not injected.

| Variable | Terraform output |
| --- | --- |
| `REVIEW_HARNESS_CLOUDSTORAGE_BUCKET` | `cloudstorage_bucket` |
| `GCP_SERVICE_ACCOUNT_EMAIL` | `github_actions_service_account_email` |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | `workload_identity_provider` |

Set these repository secrets for API access:

| Secret | Purpose |
| --- | --- |
| `ANTHROPIC_API_KEY` | Required for Claude Code |
| `OPENAI_API_KEY` | Optional for embeddings |

## GitHub Actions auth

Use the Terraform outputs with `google-github-actions/auth@v2`:

```yaml
permissions:
  contents: read
  pull-requests: write
  id-token: write

steps:
  - uses: google-github-actions/auth@v2
    with:
      workload_identity_provider: ${{ vars.GCP_WORKLOAD_IDENTITY_PROVIDER }}
      service_account: ${{ vars.GCP_SERVICE_ACCOUNT_EMAIL }}
```
