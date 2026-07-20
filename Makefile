REGISTRY ?= 
REPO ?= 
IMAGE := $(REGISTRY)/$(REPO)

GIT_SHA := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD | sed 's/[^A-Za-z0-9_.-]/-/g')
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION ?= $(GIT_SHA)

DOCKER_BUILD_ARGS := \
	--build-arg BUILD_DATE=$(BUILD_DATE) \
	--build-arg VERSION=$(VERSION) \
	--build-arg GIT_SHA=$(GIT_SHA)

TERRAFORM_DIR ?= terraform

CDN_DISTRIBUTION_ID := $(shell terraform -chdir=$(TERRAFORM_DIR) output -raw cdn_distribution_id 2>/dev/null)

.PHONY: start stop logs db-shell fmt test build assets deploy-assets help
start: fmt
	docker compose -f docker-compose.yml up -d --build
stop:
	docker compose -f docker-compose.yml down
logs:
	docker compose -f docker-compose.yml logs -f
db-shell:
	docker compose -f docker-compose.yml exec db psql -U gophoto -d gophoto
fmt:
	go fmt ./...
test: fmt
	go test -v ./...
build:
	docker build \
		$(DOCKER_BUILD_ARGS) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):$(GIT_BRANCH) \
		-t $(IMAGE):latest \
		--platform linux/amd64 \
		--push \
		.
	@echo "Built $(IMAGE):$(VERSION)"
assets:
	esbuild node_modules/bootstrap/dist/css/bootstrap.min.css \
		--bundle --minify --outfile=assets/dist/css/bootstrap.min.css
	esbuild node_modules/bootstrap/dist/js/bootstrap.bundle.min.js \
		--bundle=false --minify --outfile=assets/dist/js/bootstrap.bundle.min.js
	esbuild assets/js/main.js assets/js/photo_upload.js \
		--bundle=false \
		--minify \
		--sourcemap \
		--outdir=assets/dist/js
	esbuild assets/css/styles.css \
		--minify\
		--outdir=assets/dist/css
deploy-assets: assets
	terraform -chdir=$(TERRAFORM_DIR) apply -target=aws_s3_object.assets_css -target=aws_s3_object.assets_js -target=aws_s3_object.assets_images
	@if [ -n "$(CDN_DISTRIBUTION_ID)" ]; then \
		echo "Invalidating CloudFront distribution $(CDN_DISTRIBUTION_ID)..."; \
		aws cloudfront create-invalidation --distribution-id $(CDN_DISTRIBUTION_ID) --paths "/css/*" "/js/*" "/images/*"; \
	else \
		echo "Warning: could not resolve CDN_DISTRIBUTION_ID; skipping invalidation"; \
	fi
deploy:
	terraform -chdir=$(TERRAFORM_DIR) init
	terraform -chdir=$(TERRAFORM_DIR) fmt
	terraform -chdir=$(TERRAFORM_DIR) apply -auto-approve
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  start         Format code and start services with Docker Compose"
	@echo "  stop          Stop services with Docker Compose"
	@echo "  logs          Follow logs from Docker Compose services"
	@echo "  db-shell      Open a psql shell in the db container"
	@echo "  fmt           Format Go source files"
	@echo "  test          Format and run Go tests"
	@echo "  build         Build and push Docker image with version tags"
	@echo "  assets        Build the static assets"
	@echo "  deploy-assets Build assets, upload to S3, and invalidate CloudFront cache"
	@echo "  deploy        Build infrastructure with Terraform"
	@echo "  help          Show this help message"
