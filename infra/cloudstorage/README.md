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

## GitHub Secrets

Set these repository secrets from the Terraform outputs:

| Secret | Terraform output |
| --- | --- |
| `REVIEW_HARNESS_CLOUDSTORAGE_BUCKET` | `cloudstorage_bucket` |
| `GCP_SERVICE_ACCOUNT_EMAIL` | `github_actions_service_account_email` |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | `workload_identity_provider` |

`ANTHROPIC_API_KEY` is required for Claude Code. `OPENAI_API_KEY` remains optional for embeddings.

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
      workload_identity_provider: ${{ secrets.GCP_WORKLOAD_IDENTITY_PROVIDER }}
      service_account: ${{ secrets.GCP_SERVICE_ACCOUNT_EMAIL }}
```
