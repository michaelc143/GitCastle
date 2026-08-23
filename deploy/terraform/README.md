# GitCastle Terraform

Infrastructure as code for the production deployment.

## What it provisions

| Resource | Purpose |
|---|---|
| VPC + 2 private subnets | Network isolation for the database |
| RDS PostgreSQL 16 (Multi-AZ) | Managed database with encrypted storage, autoscaling to 200 GB, PITR backups |
| S3 bucket | Repository backup bundles; versioned, SSE, lifecycle-expired after the retention window |
| Random passwords | DB password + `GITCASTLE_INTERNAL_TOKEN` + secret-encryption key |

## Usage

```bash
cd deploy/terraform
terraform init
terraform plan -var environment=staging
terraform apply
```

State lives in S3 (`key = "gitcastle/terraform.tfstate"`); configure your
bucket/region in the backend block before the first apply.

## Secrets handling

Terraform marks all credential outputs `sensitive`. The deploy pipeline reads
them from Terraform outputs and writes them into the runtime secret store —
they are never committed. Rotate by `terraform apply`ing after a
`taint random_password.internal_token`.

## Mapping to GitCastle runtime configuration

| Terraform output | Runtime env var |
|---|---|
| `database_endpoint` (+password) | `DATABASE_URL` |
| `internal_token` | `GITCASTLE_INTERNAL_TOKEN` |
| `backup_bucket` | consumed by the S3 ObjectStore implementation |
| `secret_encryption_key_hex` | `SECRET_ENCRYPTION_KEY` |

The container image itself is built by CI (`backend.yml`) and deployed via
`deploy/k8s`; this module provides everything those manifests point at.
