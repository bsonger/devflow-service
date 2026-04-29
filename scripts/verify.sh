#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_CACHE_DIR="${GOCACHE:-$ROOT_DIR/.cache/go-build}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

info() {
  echo "INFO: $*"
}

run_docker_policy_check() {
  info "Running Docker policy checks"
  (
    cd "$ROOT_DIR"
    bash scripts/check-docker-policy.sh
  )
}

run_service_layer_db_policy_check() {
  info "Running service layer DB policy checks"
  local pattern='DB\(\)\.(ExecContext|QueryContext|QueryRowContext)|Postgres\(\)\.(ExecContext|QueryContext|QueryRowContext)'
  local transport_pattern='github.com/gin-gonic/gin|internal/platform/httpx|internal/.*/transport/http'
  local generic_validation_pattern='return errors\.New\(".* is required"\)|return errors\.New\(strings\.Join\(|return fmt\.Errorf\(".* is required"\)|return fmt\.Errorf\(".* not found"\)|return fmt\.Errorf\(".* invalid"\)'
  local matches
  local transport_matches
  local validation_matches
  matches="$(
    cd "$ROOT_DIR"
    rg -n "$pattern" internal/*/service --glob '!**/*_test.go' || true
  )"
  [[ -z "$matches" ]] || fail "service layer must not access DB directly; move persistence into repository:\n$matches"

  transport_matches="$(
    cd "$ROOT_DIR"
    rg -n "$transport_pattern" internal/*/service --glob '!**/*_test.go' || true
  )"
  [[ -z "$transport_matches" ]] || fail "service layer must not depend on Gin, platform/httpx, or transport/http packages:\n$transport_matches"

  validation_matches="$(
    cd "$ROOT_DIR"
    rg -n "$generic_validation_pattern" internal/*/service --glob '!**/*_test.go' || true
  )"
  [[ -z "$validation_matches" ]] || fail "service layer generic validation errors should use internal/shared/errs instead of ad-hoc errors.New strings:\n$validation_matches"
}

run_repository_layer_policy_check() {
  info "Running repository layer policy checks"
  local forbidden_import_pattern='github.com/gin-gonic/gin|internal/platform/httpx|internal/.*/transport/http|internal/.*/service'
  local constructor_pattern='func New[A-Za-z0-9]*PostgresStore\(\) \*'
  local generic_validation_pattern='return errors\.New\(".* is required"\)|return errors\.New\(strings\.Join\(|return fmt\.Errorf\(".* is required"\)|return fmt\.Errorf\(".* not found"\)|return fmt\.Errorf\(".* invalid"\)'
  local forbidden_matches
  local constructor_matches
  local validation_matches

  forbidden_matches="$(
    cd "$ROOT_DIR"
    rg -n "$forbidden_import_pattern" internal/*/repository --glob '!**/*_test.go' || true
  )"
  [[ -z "$forbidden_matches" ]] || fail "repository layer must not depend on Gin, platform/httpx, transport/http, or service packages:
$forbidden_matches"

  constructor_matches="$(
    cd "$ROOT_DIR"
    rg -n "$constructor_pattern" internal/*/repository --glob '!**/*_test.go' || true
  )"
  [[ -z "$constructor_matches" ]] || fail "repository constructors should return interface types instead of concrete PostgreSQL stores:
$constructor_matches"

  validation_matches="$(
    cd "$ROOT_DIR"
    rg -n "$generic_validation_pattern" internal/*/repository --glob '!**/*_test.go' || true
  )"
  [[ -z "$validation_matches" ]] || fail "repository generic validation errors should use internal/shared/errs instead of ad-hoc errors.New strings:
$validation_matches"
}

run_downstream_client_policy_check() {
  info "Running downstream client policy checks"
  local raw_http_pattern='http\.Client\{|http\.NewRequestWithContext\('
  local ad_hoc_matches

  ad_hoc_matches="$(
    cd "$ROOT_DIR"
    rg -n "$raw_http_pattern" internal/*/service internal/*/support internal/*/runtime --glob '!**/*_test.go' || true
  )"
  [[ -z "$ad_hoc_matches" ]] || fail "HTTP-based downstream calls outside dedicated downstream adapters should reuse internal/shared/downstreamhttp or transport/downstream packages:
$ad_hoc_matches"
}

run_worker_runtime_policy_check() {
  info "Running worker/runtime policy checks"
  local forbidden_import_pattern='github.com/gin-gonic/gin|internal/platform/httpx|internal/.*/transport/http'
  local matches

  matches="$(
    cd "$ROOT_DIR"
    rg -n "$forbidden_import_pattern" internal/platform/runtime internal/release/runtime --glob '!**/*_test.go' || true
  )"
  [[ -z "$matches" ]] || fail "worker/runtime helper packages must not depend on Gin, platform/httpx, or transport/http packages:
$matches"
}

run_no_mongo_remnants_check() {
  info "Running Mongo removal checks"
  local pattern='go\.mongodb\.org/mongo-driver|mongo-driver|mongodb|primitive\.ObjectID|\bObjectID\b|\bbson\b'
  local matches
  matches="$(
    cd "$ROOT_DIR"
    rg -n -i "$pattern" \
      go.mod \
      README.md \
      docs \
      internal \
      cmd \
      scripts \
      deployments \
      api \
      gateway \
      test \
      --glob '!docs/archive/**' \
      --glob '!docs/policies/verification.md' \
      --glob '!scripts/README.md' \
      --glob '!scripts/verify.sh' \
      --glob '!**/*_test.go' || true
  )"
  [[ -z "$matches" ]] || fail "Mongo-era references remain after PostgreSQL migration:\n$matches"
}

run_observability_logging_policy_check() {
  info "Running observability logging policy checks"

  local dotted_pattern='zap\.(String|Int|Int32|Int64|Bool|Duration|Float64|Any)\("[^"]*\.[^"]*"'
  local camel_pattern='zap\.(String|Int|Int32|Int64|Bool|Duration|Float64|Any)\("[^"]*[A-Z][^"]*"'
  local dotted_matches
  local camel_matches

  dotted_matches="$(
    cd "$ROOT_DIR"
    rg -n "$dotted_pattern" internal cmd --glob '!**/*_test.go' || true
  )"
  [[ -z "$dotted_matches" ]] || fail "structured log field names must not use dotted keys; use snake_case instead:\n$dotted_matches"

  camel_matches="$(
    cd "$ROOT_DIR"
    rg -n "$camel_pattern" internal cmd --glob '!**/*_test.go' || true
  )"
  [[ -z "$camel_matches" ]] || fail "structured log field names must not use camelCase or uppercase letters; use snake_case instead:\n$camel_matches"
}

run_metrics_label_policy_check() {
  info "Running metrics label policy checks"

  local forbidden_metric_label_pattern='attribute\.String\("(trace_id|request_id|user_id|email|phone|order_id|release_id|session_id|token|authorization|cookie)"'
  local matches

  matches="$(
    cd "$ROOT_DIR"
    rg -n "$forbidden_metric_label_pattern" internal cmd --glob '!**/*_test.go' || true
  )"
  [[ -z "$matches" ]] || fail "metrics attributes must not use high-cardinality or sensitive identifier labels:\n$matches"
}

run_http_handler_uuid_policy_check() {
  info "Running HTTP handler UUID parsing checks"

  local matches
  matches="$(
    cd "$ROOT_DIR"
    rg -n 'uuid\.Parse\(' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$matches" ]] || fail "HTTP handlers should use shared httpx UUID parse helpers instead of hand-rolled uuid.Parse blocks:\n$matches"
}

run_http_api_selector_policy_check() {
  info "Running HTTP API selector policy checks"
  (
    cd "$ROOT_DIR"
    bash scripts/check-http-api-selector-policy.sh
  )
}

run_http_handler_helper_policy_check() {
  info "Running HTTP handler helper usage checks"

  local bind_matches
  local pagination_matches
  local internal_error_matches
  local invalid_argument_matches
  local failed_precondition_matches
  local unauthorized_matches

  bind_matches="$(
    cd "$ROOT_DIR"
    rg -n 'ShouldBindJSON\(' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$bind_matches" ]] || fail "HTTP handlers should use httpx.BindJSON instead of direct ShouldBindJSON calls:\n$bind_matches"

  pagination_matches="$(
    cd "$ROOT_DIR"
    rg -n 'ParsePagination\(c\)|PaginateSlice\(|WriteList\(' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$pagination_matches" ]] || fail "HTTP handlers should use shared httpx pagination helpers instead of hand-rolled list pagination flows:\n$pagination_matches"

  internal_error_matches="$(
    cd "$ROOT_DIR"
    rg -n 'WriteError\(c, http\.StatusInternalServerError, "internal", err\.Error\(\), nil\)' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$internal_error_matches" ]] || fail "HTTP handlers must not return raw err.Error() in internal 5xx responses; use httpx.WriteInternalError:\n$internal_error_matches"

  invalid_argument_matches="$(
    cd "$ROOT_DIR"
    rg -n 'WriteError\(c, http\.StatusBadRequest, "invalid_argument", .*?, nil\)' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$invalid_argument_matches" ]] || fail "HTTP handlers should use httpx.WriteInvalidArgument instead of generic WriteError for invalid_argument responses:\n$invalid_argument_matches"

  failed_precondition_matches="$(
    cd "$ROOT_DIR"
    rg -n 'WriteError\(c, http\.(StatusConflict|StatusFailedDependency), "failed_precondition", .*?, nil\)' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$failed_precondition_matches" ]] || fail "HTTP handlers should use httpx.WriteFailedPrecondition instead of generic WriteError for failed_precondition responses:\n$failed_precondition_matches"

  unauthorized_matches="$(
    cd "$ROOT_DIR"
    rg -n 'WriteError\(c, http\.StatusUnauthorized, "unauthorized", "unauthorized", nil\)' internal/*/transport/http --glob '!**/*_test.go' || true
  )"
  [[ -z "$unauthorized_matches" ]] || fail "HTTP handlers should use httpx.WriteUnauthorized instead of repeating raw unauthorized envelopes:\n$unauthorized_matches"
}

run_layout_refactor_policy_check() {
  info "Running layout refactor policy checks"

  local alias_files
  local bad_shared_dirs
  local platform_business_imports
  alias_files="$(
    cd "$ROOT_DIR"
    find internal -name 'support_alias.go' -print | sort
  )"
  [[ -z "$alias_files" ]] || fail "alias-only forwarding files must not be reintroduced; import the owning package directly instead:\n$alias_files"

  bad_shared_dirs="$(
    cd "$ROOT_DIR"
    find internal/shared -type d \( -name common -o -name util -o -name utils -o -name base -o -name model \) -print | sort || true
  )"
  [[ -z "$bad_shared_dirs" ]] || fail "catch-all shared directory names must not exist under internal/shared:\n$bad_shared_dirs"

  platform_business_imports="$(
    cd "$ROOT_DIR"
    rg -n 'github.com/bsonger/devflow-service/internal/(application|project|cluster|environment|appconfig|service|route|workloadconfig|image|manifest|intent|release)' internal/platform internal/shared --glob '!**/*_test.go' || true
  )"
  [[ -z "$platform_business_imports" ]] || fail "internal/platform and internal/shared must not import business-domain packages directly:\n$platform_business_imports"
}

run_runtime_no_postgres_policy_check() {
  info "Running runtime no-Postgres policy checks"

  local pattern='NewPostgresStore\(|platformdb\.Postgres\('
  local matches

  matches="$(
    cd "$ROOT_DIR"
    rg -n "$pattern" internal/runtime --glob '!**/*_test.go' || true
  )"
  [[ -z "$matches" ]] || fail "runtime code must not reintroduce NewPostgresStore() or platformdb.Postgres() outside tests:\n$matches"
}

require_file() {
  local path="$1"
  local label="$2"
  [[ -f "$path" ]] || fail "$label is missing: $path"
  [[ -s "$path" ]] || fail "$label is empty: $path"
}

require_dir() {
  local path="$1"
  local label="$2"
  [[ -d "$path" ]] || fail "$label is missing: $path"
}

require_literal() {
  local path="$1"
  local label="$2"
  local needle="$3"
  grep -Fq "$needle" "$path" || fail "$label missing from $(basename "$path"): expected literal [$needle]"
}

run_go_test() {
  info "Running go test ./..."
  (
    cd "$ROOT_DIR"
    mkdir -p "$GO_CACHE_DIR"
    GOCACHE="$GO_CACHE_DIR" go test ./...
  )
}

run_go_build_targets() {
  info "Building root service entrypoints"
  (
    cd "$ROOT_DIR"
    mkdir -p bin
    GOCACHE="$GO_CACHE_DIR" go build -o bin/meta-service ./cmd/meta-service
    GOCACHE="$GO_CACHE_DIR" go build -o bin/config-service ./cmd/config-service
    GOCACHE="$GO_CACHE_DIR" go build -o bin/network-service ./cmd/network-service
    GOCACHE="$GO_CACHE_DIR" go build -o bin/release-service ./cmd/release-service
    GOCACHE="$GO_CACHE_DIR" go build -o bin/runtime-service ./cmd/runtime-service
  )
}

info "Checking repository-local root-module and recovery surfaces"
require_file "$ROOT_DIR/go.mod" "root go.mod"
require_file "$ROOT_DIR/.gitignore" "root .gitignore"
require_file "$ROOT_DIR/README.md" "root README"
require_file "$ROOT_DIR/AGENTS.md" "root AGENTS"
require_file "$ROOT_DIR/docs/README.md" "docs index"
require_file "$ROOT_DIR/docs/index/README.md" "docs index README"
require_file "$ROOT_DIR/docs/index/getting-started.md" "docs getting started"
require_file "$ROOT_DIR/docs/index/agent-path.md" "docs agent path"
require_file "$ROOT_DIR/docs/system/architecture.md" "system architecture doc"
require_file "$ROOT_DIR/docs/system/constraints.md" "system constraints doc"
require_file "$ROOT_DIR/docs/system/observability.md" "system observability doc"
require_file "$ROOT_DIR/docs/system/release-writeback.md" "system release writeback doc"
require_file "$ROOT_DIR/docs/system/recovery.md" "system recovery doc"
require_file "$ROOT_DIR/docs/services/meta-service.md" "meta-service service doc"
require_file "$ROOT_DIR/docs/services/release-service.md" "release-service service doc"
require_file "$ROOT_DIR/docs/resources/README.md" "resources README"
require_file "$ROOT_DIR/docs/resources/application.md" "application resource doc"
require_file "$ROOT_DIR/docs/resources/project.md" "project resource doc"
require_file "$ROOT_DIR/docs/resources/cluster.md" "cluster resource doc"
require_file "$ROOT_DIR/docs/resources/environment.md" "environment resource doc"
require_file "$ROOT_DIR/docs/resources/appconfig.md" "appconfig resource doc"
require_file "$ROOT_DIR/docs/resources/workloadconfig.md" "workloadconfig resource doc"
require_file "$ROOT_DIR/docs/resources/runtime-spec.md" "runtime-spec resource doc"
require_file "$ROOT_DIR/docs/resources/service.md" "service resource doc"
require_file "$ROOT_DIR/docs/resources/route.md" "route resource doc"
require_file "$ROOT_DIR/docs/resources/image.md" "image resource doc"
require_file "$ROOT_DIR/docs/resources/manifest.md" "manifest resource doc"
require_file "$ROOT_DIR/docs/resources/intent.md" "intent resource doc"
require_file "$ROOT_DIR/docs/resources/release.md" "release resource doc"
require_file "$ROOT_DIR/docs/policies/go-monorepo-layout.md" "go monorepo layout policy doc"
require_file "$ROOT_DIR/docs/policies/docker-baseline.md" "docker baseline policy doc"
require_file "$ROOT_DIR/docs/policies/verification.md" "verification policy doc"
require_file "$ROOT_DIR/docs/policies/observability-logging.md" "observability logging policy doc"
require_file "$ROOT_DIR/docs/policies/error-handling.md" "error handling policy doc"
require_file "$ROOT_DIR/docs/policies/http-handler.md" "http handler policy doc"
require_file "$ROOT_DIR/docs/policies/service-layer.md" "service layer policy doc"
require_file "$ROOT_DIR/docs/policies/downstream-client.md" "downstream client policy doc"
require_file "$ROOT_DIR/docs/policies/repository-layer.md" "repository layer policy doc"
require_file "$ROOT_DIR/docs/policies/worker-runtime.md" "worker runtime policy doc"
require_file "$ROOT_DIR/docs/policies/resource-api.md" "resource api policy doc"
require_file "$ROOT_DIR/scripts/README.md" "scripts README"
require_file "$ROOT_DIR/scripts/verify.sh" "repo-local verifier"
require_file "$ROOT_DIR/scripts/docker-build.sh" "repo-local docker build wrapper"
require_file "$ROOT_DIR/scripts/regen-swagger.sh" "meta-service swagger regen script"
require_file "$ROOT_DIR/scripts/check-docker-policy.sh" "Docker policy checker"
require_file "$ROOT_DIR/scripts/check-docker-policy_test.sh" "Docker policy checker test"
require_file "$ROOT_DIR/docs/docker.md" "Docker redirect doc"
require_dir "$ROOT_DIR/docker" "docker assets directory"
require_file "$ROOT_DIR/docker/README.md" "docker assets README"
require_file "$ROOT_DIR/docker/golang-builder.Dockerfile" "repo-local golang builder Dockerfile"
require_file "$ROOT_DIR/docker/service.Dockerfile.template" "service Dockerfile template"
require_file "$ROOT_DIR/deployments/pre-production/meta-service.yaml" "meta-service pre-production deployment manifest"
require_file "$ROOT_DIR/deployments/pre-production/config-service.yaml" "config-service pre-production deployment manifest"
require_file "$ROOT_DIR/deployments/pre-production/network-service.yaml" "network-service pre-production deployment manifest"
require_file "$ROOT_DIR/deployments/pre-production/runtime-service.yaml" "runtime-service pre-production deployment manifest"
require_file "$ROOT_DIR/deployments/pre-production/istio/shared-ingress.yaml" "shared Istio pre-production ingress manifest"
require_file "$ROOT_DIR/gateway/README.md" "gateway README"
require_dir "$ROOT_DIR/docs/generated" "generated docs directory"
require_dir "$ROOT_DIR/docs/archive" "archive docs directory"
require_dir "$ROOT_DIR/docs/resources" "resources docs directory"

require_dir "$ROOT_DIR/cmd" "cmd directory"
require_dir "$ROOT_DIR/internal" "internal directory"
require_dir "$ROOT_DIR/api" "api directory"
require_dir "$ROOT_DIR/deployments" "deployments directory"
require_dir "$ROOT_DIR/test" "test directory"
require_dir "$ROOT_DIR/gateway" "gateway directory"

require_dir "$ROOT_DIR/internal/app" "internal app assembly directory"
require_dir "$ROOT_DIR/internal/platform" "internal platform directory"
require_dir "$ROOT_DIR/internal/shared" "internal shared directory"
require_dir "$ROOT_DIR/internal/shared/downstreamhttp" "shared downstream http directory"
require_dir "$ROOT_DIR/internal/platform/config" "internal platform config directory"
require_dir "$ROOT_DIR/internal/platform/db" "internal platform db directory"
require_file "$ROOT_DIR/internal/app/router.go" "internal app router"
require_file "$ROOT_DIR/internal/app/router_test.go" "internal app router tests"
require_file "$ROOT_DIR/internal/platform/config/config.go" "internal platform config"
require_file "$ROOT_DIR/internal/platform/db/postgres.go" "internal platform db"
require_file "$ROOT_DIR/cmd/release-service/main.go" "release-service entrypoint"
require_file "$ROOT_DIR/cmd/config-service/main.go" "config-service entrypoint"
require_file "$ROOT_DIR/cmd/network-service/main.go" "network-service entrypoint"
require_file "$ROOT_DIR/cmd/runtime-service/main.go" "runtime-service entrypoint"
require_dir "$ROOT_DIR/internal/configservice" "internal configservice directory"
require_dir "$ROOT_DIR/internal/networkservice" "internal networkservice directory"
require_dir "$ROOT_DIR/internal/runtime" "internal runtime directory"
require_dir "$ROOT_DIR/internal/application/transport/downstream" "application downstream transport directory"
require_dir "$ROOT_DIR/internal/project/transport/downstream" "project downstream transport directory"
require_dir "$ROOT_DIR/internal/environment/transport/downstream" "environment downstream transport directory"
require_dir "$ROOT_DIR/internal/cluster/transport/downstream" "cluster downstream transport directory"
require_dir "$ROOT_DIR/internal/manifest" "internal manifest directory"
require_file "$ROOT_DIR/internal/manifest/module.go" "manifest module"
require_dir "$ROOT_DIR/internal/manifest/service" "manifest service directory"
require_dir "$ROOT_DIR/internal/manifest/transport/http" "manifest http transport directory"
require_dir "$ROOT_DIR/internal/appconfig/transport/downstream" "appconfig downstream transport directory"
require_dir "$ROOT_DIR/internal/service/transport/downstream" "service downstream transport directory"
require_dir "$ROOT_DIR/internal/intent" "internal intent directory"
require_file "$ROOT_DIR/internal/intent/module.go" "intent module"
require_dir "$ROOT_DIR/internal/intent/service" "intent service directory"
require_dir "$ROOT_DIR/internal/intent/transport/http" "intent http transport directory"
require_dir "$ROOT_DIR/internal/release" "internal release directory"
require_file "$ROOT_DIR/internal/release/module.go" "release module"
require_file "$ROOT_DIR/internal/release/transport/http/router.go" "release router"
require_file "$ROOT_DIR/internal/release/config/config.go" "release config"
require_dir "$ROOT_DIR/internal/release/domain" "release domain directory"
require_dir "$ROOT_DIR/internal/release/support" "release support directory"
require_dir "$ROOT_DIR/internal/release/service" "release service directory"
require_dir "$ROOT_DIR/internal/platform/k8s" "internal platform k8s directory"
require_dir "$ROOT_DIR/internal/release/transport" "release transport directory"
require_dir "$ROOT_DIR/internal/release/transport/argo" "release argo transport directory"
require_dir "$ROOT_DIR/internal/release/transport/downstream" "release downstream transport directory"
require_dir "$ROOT_DIR/internal/release/transport/tekton" "release tekton transport directory"

[[ ! -d "$ROOT_DIR/shared" ]] || fail "catch-all shared directory must not exist: $ROOT_DIR/shared"
[[ ! -d "$ROOT_DIR/common" ]] || fail "catch-all common directory must not exist: $ROOT_DIR/common"
[[ ! -d "$ROOT_DIR/util" ]] || fail "catch-all util directory must not exist: $ROOT_DIR/util"
[[ ! -d "$ROOT_DIR/modules" ]] || fail "legacy modules directory must not exist: $ROOT_DIR/modules"
[[ ! -d "$ROOT_DIR/internal/release/argoclient" ]] || fail "legacy release argoclient directory must not exist: $ROOT_DIR/internal/release/argoclient"
[[ ! -d "$ROOT_DIR/internal/release/downstream" ]] || fail "legacy release downstream directory must not exist: $ROOT_DIR/internal/release/downstream"
[[ ! -d "$ROOT_DIR/internal/release/runtimeclient" ]] || fail "legacy release runtimeclient directory must not exist: $ROOT_DIR/internal/release/runtimeclient"
[[ ! -d "$ROOT_DIR/internal/release/infra" ]] || fail "legacy release infra directory must not exist: $ROOT_DIR/internal/release/infra"
[[ ! -d "$ROOT_DIR/internal/release/router" ]] || fail "legacy release router directory must not exist: $ROOT_DIR/internal/release/router"
[[ ! -d "$ROOT_DIR/internal/release/api" ]] || fail "legacy release api directory must not exist: $ROOT_DIR/internal/release/api"
[[ ! -d "$ROOT_DIR/internal/release/model" ]] || fail "legacy release model directory must not exist: $ROOT_DIR/internal/release/model"
[[ ! -d "$ROOT_DIR/internal/release/store" ]] || fail "legacy release store directory must not exist: $ROOT_DIR/internal/release/store"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/app.go" ]] || fail "legacy release app downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/app.go"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/config_manifest.go" ]] || fail "legacy release config downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/config_manifest.go"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/network_manifest.go" ]] || fail "legacy release network downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/network_manifest.go"
[[ ! -f "$ROOT_DIR/internal/release/transport/downstream/client.go" ]] || fail "legacy release shared downstream client must not exist: $ROOT_DIR/internal/release/transport/downstream/client.go"

info "Checking root module contract"
require_literal "$ROOT_DIR/go.mod" "module path" "module github.com/bsonger/devflow-service"
require_literal "$ROOT_DIR/go.mod" "go baseline" "go 1.26.2"

info "Checking root entrypoint wiring"
require_literal "$ROOT_DIR/README.md" "README recovery link" "docs/system/recovery.md"
require_literal "$ROOT_DIR/README.md" "README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/AGENTS.md" "AGENTS recovery link" "docs/system/recovery.md"
require_literal "$ROOT_DIR/docs/README.md" "docs index agent start" "AGENTS.md"
require_literal "$ROOT_DIR/docs/index/README.md" "index README canonical agent start" "AGENTS.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/policies/docker-baseline.md" "Docker baseline install ban" "go install"
require_literal "$ROOT_DIR/docs/policies/docker-baseline.md" "Docker baseline go version" "1.26.2"
require_literal "$ROOT_DIR/docker/README.md" "docker assets README verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docker/README.md" "docker assets README policy reference" "approved FROM references"
require_literal "$ROOT_DIR/README.md" "README release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/README.md" "README config-service build target" "./cmd/config-service"
require_literal "$ROOT_DIR/README.md" "README network-service build target" "./cmd/network-service"
require_literal "$ROOT_DIR/README.md" "README runtime-service build target" "./cmd/runtime-service"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery config-service build target" "./cmd/config-service"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery network-service build target" "./cmd/network-service"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery runtime-service build target" "./cmd/runtime-service"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy config-service build target" "./cmd/config-service"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy network-service build target" "./cmd/network-service"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy runtime-service build target" "./cmd/runtime-service"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README release-service build target" "./cmd/release-service"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README config-service build target" "./cmd/config-service"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README network-service build target" "./cmd/network-service"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README runtime-service build target" "./cmd/runtime-service"
require_literal "$ROOT_DIR/cmd/README.md" "cmd README config-service entrypoint" '`cmd/config-service`'
require_literal "$ROOT_DIR/cmd/README.md" "cmd README network-service entrypoint" '`cmd/network-service`'
require_literal "$ROOT_DIR/cmd/README.md" "cmd README runtime-service entrypoint" '`cmd/runtime-service`'
require_literal "$ROOT_DIR/docs/services/config-service.md" "config-service entrypoint doc" '`cmd/config-service/main.go`'
require_literal "$ROOT_DIR/docs/services/network-service.md" "network-service entrypoint doc" '`cmd/network-service/main.go`'
require_literal "$ROOT_DIR/docs/services/runtime-service.md" "runtime-service entrypoint doc" '`cmd/runtime-service/main.go`'
require_literal "$ROOT_DIR/docs/resources/runtime-spec.md" "runtime API surface workload" "/api/v1/runtime/workload"
require_literal "$ROOT_DIR/docs/resources/runtime-spec.md" "runtime API surface pods" "/api/v1/runtime/pods/{pod_name}"
require_literal "$ROOT_DIR/gateway/README.md" "gateway README Istio contract" "Istio-oriented"
require_literal "$ROOT_DIR/gateway/README.md" "gateway README shared ingress manifest" "deployments/pre-production/istio/shared-ingress.yaml"
require_literal "$ROOT_DIR/gateway/README.md" "gateway README shared host" "devflow-pre-production.bei.com"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service release support area" "internal/release/support"

info "Checking repo-local documentation alignment"
require_literal "$ROOT_DIR/README.md" "README docs layout" "docs/index/"
require_literal "$ROOT_DIR/README.md" "README recovery contract" "docs/system/recovery.md"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture cmd layout" "cmd/"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture internal layout" "internal/"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture app assembly" "internal/platform/"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture config-service entrypoint" "cmd/config-service/main.go"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture network-service entrypoint" "cmd/network-service/main.go"
require_literal "$ROOT_DIR/docs/system/architecture.md" "system architecture runtime-service entrypoint" "cmd/runtime-service/main.go"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/system/recovery.md" "system recovery migration target" "repository root layout"
require_literal "$ROOT_DIR/docs/system/observability.md" "system observability build proof" './cmd/meta-service'
require_literal "$ROOT_DIR/docs/system/observability.md" "system observability internal status" "/internal/status"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy verifier command" "bash scripts/verify.sh"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy observability policy reference" "docs/policies/observability-logging.md"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy error handling policy reference" "docs/policies/error-handling.md"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy http handler policy reference" "docs/policies/http-handler.md"
require_literal "$ROOT_DIR/docs/policies/http-handler.md" "http handler bind helper" "BindJSON"
require_literal "$ROOT_DIR/docs/policies/http-handler.md" "http handler paginated list helper" "WritePaginatedList"
require_literal "$ROOT_DIR/docs/policies/error-handling.md" "error handling internal error helper" "WriteInternalError"
require_literal "$ROOT_DIR/docs/policies/error-handling.md" "error handling shared errs helper" "internal/shared/errs"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy service layer policy reference" "docs/policies/service-layer.md"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy downstream client policy reference" "docs/policies/downstream-client.md"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy repository layer policy reference" "docs/policies/repository-layer.md"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy shared errs rule" "internal/shared/errs"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy downstream helper rule" "internal/shared/downstreamhttp"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy worker runtime policy reference" "docs/policies/worker-runtime.md"
require_literal "$ROOT_DIR/docs/policies/verification.md" "verification policy resource api policy reference" "docs/policies/resource-api.md"
require_literal "$ROOT_DIR/docs/README.md" "docs README resource api reference" "docs/policies/resource-api.md"
require_literal "$ROOT_DIR/docs/policies/README.md" "policies README resource api reference" "resource-api.md"
require_literal "$ROOT_DIR/docs/resources/README.md" "resources README resource api reference" "docs/policies/resource-api.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README resource api reference" "resource-api policy"
require_literal "$ROOT_DIR/docs/README.md" "docs README worker runtime reference" "docs/policies/worker-runtime.md"
require_literal "$ROOT_DIR/docs/policies/README.md" "policies README worker runtime reference" "worker-runtime.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README worker runtime reference" "worker-runtime policy"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service worker runtime reference" "docs/policies/worker-runtime.md"
require_literal "$ROOT_DIR/docs/README.md" "docs README repository layer reference" "docs/policies/repository-layer.md"
require_literal "$ROOT_DIR/docs/policies/README.md" "policies README repository layer reference" "repository-layer.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README repository layer reference" "repository-layer policy"
require_literal "$ROOT_DIR/docs/README.md" "docs README downstream client reference" "docs/policies/downstream-client.md"
require_literal "$ROOT_DIR/docs/policies/README.md" "policies README downstream client reference" "downstream-client.md"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README downstream client reference" "downstream-client policy"
require_literal "$ROOT_DIR/docs/policies/service-layer.md" "service layer shared errs helper" "internal/shared/errs"
require_literal "$ROOT_DIR/docs/policies/repository-layer.md" "repository layer shared errs helper" "internal/shared/errs"
require_literal "$ROOT_DIR/docs/policies/observability-logging.md" "observability policy snake_case rule" "snake_case"
require_literal "$ROOT_DIR/scripts/README.md" "scripts README meta-service build target" "./cmd/meta-service"
require_literal "$ROOT_DIR/docs/services/config-service.md" "config-service router target" "internal/configservice/transport/http/router.go"
require_literal "$ROOT_DIR/docs/services/network-service.md" "network-service router target" "internal/networkservice/transport/http/router.go"
require_literal "$ROOT_DIR/docs/services/runtime-service.md" "runtime-service router target" "internal/runtime/transport/http"
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service internal platform target" "internal/platform/..."
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service internal status endpoint" "/internal/status"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service internal status endpoint" "/internal/status"

info "Checking meta-service documentation and packaging contract"
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service service name" "meta-service"
require_literal "$ROOT_DIR/docs/services/meta-service.md" "meta-service root build target" "./cmd/meta-service"
require_literal "$ROOT_DIR/docs/services/config-service.md" "config-service service name" "config-service"
require_literal "$ROOT_DIR/docs/services/network-service.md" "network-service service name" "network-service"
require_literal "$ROOT_DIR/docs/services/runtime-service.md" "runtime-service service name" "runtime-service"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service service name" "release-service"
require_literal "$ROOT_DIR/docs/services/release-service.md" "release-service verify merge note" "verify-service"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker builder base" "FROM registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22 AS builder"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker scratch base" "FROM scratch"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker default service arg" "ARG SERVICE_NAME=meta-service"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker default port arg" "ARG SERVICE_PORT=8081"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker build target" 'go build -o /out/service ./cmd/${SERVICE_NAME}'
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker copied binary" "COPY --from=builder /out/service ./service"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker copied certs" "COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt"
require_literal "$ROOT_DIR/Dockerfile" "meta-service Docker entrypoint" 'ENTRYPOINT ["/app/service"]'
require_literal "$ROOT_DIR/scripts/regen-swagger.sh" "meta-service optional swag guard" "swag CLI not installed; skipping Swagger regeneration"
require_literal "$ROOT_DIR/internal/app/router_test.go" "meta-service identity assertion" 'payload.Service != "meta-service"'

run_docker_policy_check
run_service_layer_db_policy_check
run_repository_layer_policy_check
run_downstream_client_policy_check
run_worker_runtime_policy_check
run_no_mongo_remnants_check
run_observability_logging_policy_check
run_metrics_label_policy_check
run_http_handler_uuid_policy_check
run_http_handler_helper_policy_check
run_http_api_selector_policy_check
run_layout_refactor_policy_check
run_runtime_no_postgres_policy_check
run_go_test
run_go_build_targets

info "Repository-local verification passed."
echo "  repo: $ROOT_DIR"
echo "  recovery: $ROOT_DIR/docs/system/recovery.md"
echo "  verifier: $ROOT_DIR/scripts/verify.sh"
