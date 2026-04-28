# review-harness Cloud Storage

Terraform for the review-harness persistent memory store on Google Cloud Storage.

## Apply

```bash
cd infra/cloudstorage
terraform init
terraform apply \
  -var='project_id=my-gcp-project' \
  -var='bucket_name=my-unique-review-harness-memory' \
  -var='github_repository=shiron-dev/ai-baton'
```

## GitHub Secrets

Set these repository secrets from the Terraform outputs:

| Secret | Terraform output |
| --- | --- |
| `REVIEW_HARNESS_CLOUDSTORAGE_BUCKET` | `cloudstorage_bucket` |
| `GCP_SERVICE_ACCOUNT_EMAIL` | `github_actions_service_account_email` |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | `workload_identity_provider` |

`ANTHROPIC_API_KEY` is required for Claude Code. `OPENAI_API_KEY` remains optional for embeddings.
