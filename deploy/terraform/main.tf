###############################################################################
# GitCastle — Terraform infrastructure
#
# Provisions the production runtime: container registry image, managed
# Postgres, object storage for backups, and the app deployment. Cloud-agnostic
# by design; the "app" module targets any Docker-capable host via generic
# resources, while storage/backup modules speak AWS S3 (swap the provider to
# target GCS/Azure).
###############################################################################

terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # State is stored remotely so CI and operators share one source of truth.
  backend "s3" {
    key = "gitcastle/terraform.tfstate"
  }
}

variable "region" {
  type        = string
  default     = "us-east-1"
  description = "Region for all resources."
}

variable "environment" {
  type        = string
  default     = "production"
  description = "Environment label applied to every resource."
}

variable "database_instance_class" {
  type        = string
  default     = "db.t4g.small"
  description = "RDS instance size."
}

variable "backup_retention_days" {
  type        = number
  default     = 30
  description = "Object-lock retention window for repository backups."
}

locals {
  name_prefix = "gitcastle-${var.environment}"
  common_tags = {
    Project     = "gitcastle"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

provider "aws" {
  region = var.region
}

# --- Networking -------------------------------------------------------------

resource "aws_vpc" "main" {
  cidr_block           = "10.20.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-vpc" })
}

resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index)
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-private-${count.index}" })
}

data "aws_availability_zones" "available" {
  state = "available"
}

# --- Database ---------------------------------------------------------------

resource "random_password" "db" {
  length  = 32
  special = false
}

resource "aws_db_subnet_group" "main" {
  name       = "${local.name_prefix}-db"
  subnet_ids = aws_subnet.private[*].id

  tags = local.common_tags
}

resource "aws_db_instance" "postgres" {
  identifier     = "${local.name_prefix}-postgres"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.database_instance_class

  db_name  = "gitcastle"
  username = "gitcastle"
  password = random_password.db.result

  allocated_storage     = 20
  max_allocated_storage = 200 # storage autoscaling ceiling
  storage_encrypted     = true

  backup_retention_period = var.backup_retention_days
  backup_window           = "03:00-04:00"
  deletion_protection     = true
  multi_az                = true

  db_subnet_group_name   = aws_db_subnet_group.main.name
  skip_final_snapshot    = false
  final_snapshot_identifier = "${local.name_prefix}-final"

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-postgres" })
}

# --- Object storage for repository backups ----------------------------------

resource "aws_s3_bucket" "backups" {
  bucket = "${local.name_prefix}-backups-${data.aws_caller_identity.current.account_id}"

  tags = merge(local.common_tags, { Purpose = "repository-backups" })
}

data "aws_caller_identity" "current" {}

resource "aws_s3_bucket_versioning" "backups" {
  bucket = aws_s3_bucket.backups.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Expire noncurrent bundle versions after the retention window.
resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id

  rule {
    id     = "expire-old-backups"
    status = "Enabled"
    filter {
      prefix = "backups/"
    }
    expiration {
      days = var.backup_retention_days
    }
  }
}

# Block every public-access vector explicitly.
resource "aws_s3_bucket_public_access_block" "backups" {
  bucket = aws_s3_bucket.backups.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# --- Secrets ---------------------------------------------------------------

resource "random_password" "internal_token" {
  length  = 48
  special = false
}

resource "random_password" "secret_encryption_key_hex" {
  length  = 64
  special = false
}

# --- Outputs consumed by the deploy pipeline -------------------------------

output "database_endpoint" {
  value     = aws_db_instance.postgres.address
  sensitive = false
}

output "database_url_secret_hint" {
  description = "Assemble DATABASE_URL from this password in your secret manager; never commit it."
  value       = "postgres://gitcastle:<password-from-secrets-manager>@${aws_db_instance.postgres.address}:5432/gitcastle?sslmode=require"
  sensitive   = true
}

output "backup_bucket" {
  value = aws_s3_bucket.backups.bucket
}

output "internal_token" {
  value     = random_password.internal_token.result
  sensitive = true
}
