#!/usr/bin/env bash
# Build linux/amd64 sub2api-local:<sha>-amd64, import into k3s on vmiss-hk,
# and Recreate default/sub2api. See docs/LOCAL_FORK_GUIDE_CN.md §6.4.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

SSH_HOST="${SSH_HOST:-vmiss-hk}"
K8S_NS="${K8S_NS:-default}"
K8S_DEPLOY="${K8S_DEPLOY:-sub2api}"
K8S_CONTAINER="${K8S_CONTAINER:-sub2api}"
HEALTH_URL="${HEALTH_URL:-http://100.123.3.103:8088/health}"
SHA="$(git rev-parse --short=9 HEAD)"
IMAGE="sub2api-local:${SHA}-amd64"
DOCKER_CONFIG_FILE="${HOME}/.docker/config.json"
DOCKER_CONFIG_BACKUP=""
TMP_DOCKERFILE=""

cleanup() {
  if [[ -n "${TMP_DOCKERFILE}" && -f "${TMP_DOCKERFILE}" ]]; then
    rm -f "${TMP_DOCKERFILE}"
  fi
  if [[ -n "${DOCKER_CONFIG_BACKUP}" && -f "${DOCKER_CONFIG_BACKUP}" ]]; then
    mv "${DOCKER_CONFIG_BACKUP}" "${DOCKER_CONFIG_FILE}"
  fi
}
trap cleanup EXIT

if [[ ! -f Dockerfile ]]; then
  echo "error: run from the sub2api repo (Dockerfile missing)" >&2
  exit 1
fi

if [[ -f "${DOCKER_CONFIG_FILE}" ]] && grep -q '"credsStore"[[:space:]]*:[[:space:]]*"osxkeychain"' "${DOCKER_CONFIG_FILE}"; then
  DOCKER_CONFIG_BACKUP="$(mktemp "${TMPDIR:-/tmp}/docker-config.XXXXXX.json")"
  cp "${DOCKER_CONFIG_FILE}" "${DOCKER_CONFIG_BACKUP}"
  python3 -c '
import json, sys
path = sys.argv[1]
with open(path) as f:
    data = json.load(f)
data.pop("credsStore", None)
with open(path, "w") as f:
    json.dump(data, f, indent="\t")
    f.write("\n")
' "${DOCKER_CONFIG_FILE}"
  echo "temporarily dropped osxkeychain credsStore (restored on exit)"
fi

TMP_DOCKERFILE="$(mktemp "${REPO_ROOT}/Dockerfile.k8s.XXXXXX")"
if head -n1 Dockerfile | grep -q '^# syntax='; then
  tail -n +2 Dockerfile > "${TMP_DOCKERFILE}"
else
  cp Dockerfile "${TMP_DOCKERFILE}"
fi

echo "building ${IMAGE} (linux/amd64, commit ${SHA})"
docker build --platform linux/amd64 \
  --build-arg NODE_IMAGE=public.ecr.aws/docker/library/node:24-alpine \
  --build-arg GOLANG_IMAGE=public.ecr.aws/docker/library/golang:1.27.0-alpine \
  --build-arg ALPINE_IMAGE=public.ecr.aws/docker/library/alpine:3.21 \
  --build-arg POSTGRES_IMAGE=public.ecr.aws/docker/library/postgres:18-alpine \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  --build-arg NPM_CONFIG_REGISTRY=https://registry.npmmirror.com \
  --build-arg VERSION="local-${SHA}" \
  --build-arg COMMIT="${SHA}" \
  -t "${IMAGE}" \
  -f "${TMP_DOCKERFILE}" \
  "${REPO_ROOT}"

echo "importing ${IMAGE} into k3s on ${SSH_HOST}"
docker save "${IMAGE}" | ssh "${SSH_HOST}" 'k3s ctr images import -'
ssh "${SSH_HOST}" "k3s ctr images ls | awk '/${SHA}-amd64/ {print}'"

echo "updating ${K8S_NS}/${K8S_DEPLOY} -> ${IMAGE}"
kubectl -n "${K8S_NS}" set image "deployment/${K8S_DEPLOY}" "${K8S_CONTAINER}=${IMAGE}"
kubectl -n "${K8S_NS}" rollout status "deployment/${K8S_DEPLOY}" --timeout=960s
kubectl -n "${K8S_NS}" get pod -l app=sub2api,component=gateway -o wide

echo "health ${HEALTH_URL}"
curl -sS -m 8 -D - -o /tmp/sub2api-health.out "${HEALTH_URL}"
echo
cat /tmp/sub2api-health.out
echo
