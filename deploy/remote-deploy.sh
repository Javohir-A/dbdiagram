#!/usr/bin/env bash
# Runs ON the server, called by CI: ./remote-deploy.sh <git-sha>
# Loads the images, starts them, waits for health, and rolls back to the
# previous version if the new one does not come up.
set -euo pipefail
cd "$(dirname "$0")"

NEW="${1:?usage: remote-deploy.sh <tag>}"
PORT="$(grep -E '^APP_PORT=' .env | cut -d= -f2 || true)"; PORT="${PORT:-8081}"
PREV="$(cat .tag 2>/dev/null || true)"

echo "==> loading images for $NEW"
gunzip -c images.tgz | docker load
rm -f images.tgz

up() { TAG="$1" docker compose -f docker-compose.prod.yml --env-file .env up -d --remove-orphans; }

healthy() {
  for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:${PORT}/v1/health" 2>/dev/null | grep -q ok; then return 0; fi
    sleep 2
  done
  return 1
}

echo "==> starting $NEW (previous: ${PREV:-none})"
up "$NEW"

if healthy; then
  echo "$NEW" > .tag
  echo "==> healthy, deployed $NEW"
else
  echo "!! $NEW did not become healthy"
  TAG="$NEW" docker compose -f docker-compose.prod.yml --env-file .env logs --tail 40 backend || true
  if [ -n "$PREV" ]; then
    echo "==> rolling back to $PREV"
    up "$PREV"
    healthy && echo "==> rolled back to $PREV" || echo "!! rollback is not healthy either"
  fi
  exit 1
fi

# Keep the current and the two previous versions of each image, drop the rest.
for img in dbdiagram-backend dbdiagram-frontend; do
  docker images "$img" --format '{{.Tag}}' | tail -n +4 | xargs -r -I{} docker rmi "$img:{}" || true
done
docker image prune -f >/dev/null
