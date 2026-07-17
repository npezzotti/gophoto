# Go-Photo

Go-Photo is a simple full-stack photo management web application written in Go. Users can register, log in, create albums, upload photos, and browse them in a responsive web UI with pagination and a lightbox viewer. Photos can be stored either on local disk or in AWS S3. A background worker processes uploads (generating variants/thumbnails) and another periodically prunes orphaned images.

## Features

- Create Albums
- Upload, organize, and browse photos
- Photo storage on local disk or AWS S3
- Secure user authentication and sessions
- Responsive web interface with pagination, flash notifications, and a lightbox for image viewing
- Background worker to prune orphaned images
- Docker Compose development environment
- AWS/Kubernetes production deployment

## Demo

![Screenshot of gophoto UI](assets/images/demo.png)


## Key technologies

| Technology | Role |
|---|---|
| Go 1.20 | Server, routing (`net/http`), HTML templates |
| PostgreSQL | Primary database; also used as session store (`scs/postgresstore`) |
| Redis | Pub/sub queue for dispatching photo processing jobs |
| AWS S3 (SDK v2) | Optional object storage backend |
| bimg / libvips | Image processing — resizing and variant generation |
| sqlc | Type-safe SQL query generation |
| golang-migrate | Database schema migrations |
| nosurf | CSRF protection |
| Docker Compose | Local development environment |
| Kubernetes | Production deployment manifests |

## Project layout

```
cmd/            Entry point (main.go)
internal/
  config/       App configuration (env-based)
  db/           Database layer — sqlc-generated queries + Repository wrapping them
  domain/       Core types: Photo, Album, User, variants, statuses, errors
  service/      Business logic — AlbumService, PhotoService, UserService
  utils/        Path helpers and test utilities
  web/          HTTP handlers, middleware, session/flash, template rendering
  workers/      Background workers (photo processor, storage cleaner)
pkg/
  forms/        Form parsing and validation
  logging/      Structured logger wrapper
  pagination/   Pagination helpers
  store/        Storage abstraction — FileStore (local disk) and S3Store
  template/     Template cache loading
templates/      HTML templates (base layout + pages + partials)
kubernetes/     Kubernetes manifests for production deployment
terraform/      Terraform infrastructure-as-code for provisioning cloud resources
```

## Getting Started

**Clone the repository:**
```bash
git clone https://github.com/npezzotti/gophoto.git
cd gophoto
```

**Build and run the Docker Compose application:**
```bash
make run
```

**Open your browser:**
Visit [http://localhost:8800/signup](http://localhost:8800/signup)

## Deploying to AWS EKS

The [`terraform/`](terraform/) directory provisions all required AWS infrastructure, and the [`Makefile`](Makefile) provides targets to build the image and deploy static assets.

### Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) ≥ 1.x
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) configured with credentials that have permission to create the resources below
- [kubectl](https://kubernetes.io/docs/tasks/tools/) for applying Kubernetes manifests
- [Docker](https://docs.docker.com/get-docker/) for building and pushing the container image

### Infrastructure overview

Running `make deploy` provisions the following resources in `us-east-1` (configurable via `TF_VAR_region`):

| Resource | Details |
|---|---|
| VPC | `10.0.0.0/16` with 3 private + 3 public subnets across AZs |
| NAT Gateway | Single NAT in the first public subnet for outbound cluster traffic |
| EKS cluster | `gophoto-cluster` (Kubernetes 1.35), nodes in private subnets, node group scaling 1–3 |
| RDS PostgreSQL | `db.t3.micro`, Postgres 15, private subnets, not publicly accessible |
| ElastiCache Redis | `cache.t3.micro`, Redis 7.1, private subnets |
| S3 bucket | `main-gophoto-bucket` — photo object storage, server-side encryption enabled |
| S3 + CloudFront | `gophoto-static-assets` bucket served via CloudFront CDN |
| EKS Pod Identity | IAM role `gophoto-s3-role` bound to the `gophoto` service account for S3 access |

### 1. Configure variables

Set the required sensitive variables before running Terraform. Edit [`terraform/terraform.auto.tfvars`](terraform/terraform.auto.tfvars) or export environment variables:

```bash
# terraform/terraform.auto.tfvars
db_password = "your-secure-password"
db_username = "gophoto"
```

Or via environment variables:

```bash
export TF_VAR_db_username=gophoto
export TF_VAR_db_password=your-secure-password
```

### 2. Provision infrastructure

```bash
make deploy
```

This runs `terraform init`, `terraform fmt`, and `terraform apply` inside the `terraform/` directory.

### 3. Build and push the Docker image

Fill in the `REGISTRY` and `REPO` variables in the Makefile, then run the Make build target.

```bash
make build
```

This builds a `linux/amd64` image tagged with the current Git SHA, branch name, and `latest`, then pushes it to the registry defined by `REGISTRY`/`REPO`. Override at the command line if needed:

```bash
make build REGISTRY=123456789012.dkr.ecr.us-east-1.amazonaws.com REPO=gophoto
```

### 4. Configure the Kubernetes manifests

Edit [`kubernetes/gophoto.yml`](kubernetes/gophoto.yml) and fill in the empty values:

| Field | Where to get it |
|---|---|
| `image` | The image tag pushed in step 3 |
| `GOPHOTO_BUCKET_NAME` | `main-gophoto-bucket` (or your overridden bucket name) |
| `GOPHOTO_ASSET_BASE_URL` | `terraform -chdir=terraform output -raw cdn_domain_name` |
| `GOPHOTO_DSN` (Secret) | RDS endpoint — `aws rds describe-db-instances --db-instance-identifier main` |
| `GOPHOTO_REDIS_ADDR` (Secret) | ElastiCache endpoint — `aws elasticache describe-cache-clusters --cluster-id gophoto-redis --show-cache-node-info` |
| `GOPHOTO_SIGNING_KEY` (Secret) | A random secret string used for session signing |

### 5. Point kubectl at the EKS cluster

```bash
aws eks update-kubeconfig --region us-east-1 --name gophoto-cluster
```

### 6. Apply the Kubernetes manifests

```bash
kubectl apply -f kubernetes/gophoto.yml
```

This creates the `gophoto` Deployment, ServiceAccount, Service (type `LoadBalancer`), and Secret in the `default` namespace. The ServiceAccount name matches the EKS Pod Identity association created by Terraform, so pods automatically receive credentials to access S3.

### 7. Deploy static assets

Build and upload static assets to S3, then invalidate the CloudFront cache:

```bash
make deploy-assets
```

### Teardown

```bash
terraform -chdir=terraform destroy
```
