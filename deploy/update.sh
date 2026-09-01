#!/usr/bin/env bash
# Подтягивает новую версию бота и перезапускает контейнер, если она появилась.
# Запускается systemd-таймером (см. deploy/README.md), но можно и руками.
set -euo pipefail

cd "$(dirname "$(readlink -f "$0")")/.."

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }

# 1. Обновляем сам репозиторий - в нём лежит docker-compose.yml, который тоже
#    может меняться (новые переменные окружения, настройки логов и т.п.).
before_commit=$(git rev-parse HEAD)
git fetch --quiet origin
git reset --hard --quiet "origin/$(git rev-parse --abbrev-ref HEAD)"
after_commit=$(git rev-parse HEAD)

if [ "$before_commit" != "$after_commit" ]; then
  log "репозиторий обновлён: ${before_commit:0:7} -> ${after_commit:0:7}"
fi

# 2. Тянем образ и смотрим, изменился ли он на самом деле.
image=$(docker compose config --images | head -1)
before_image=$(docker image inspect --format '{{.Id}}' "$image" 2>/dev/null || echo "none")

docker compose pull --quiet

after_image=$(docker image inspect --format '{{.Id}}' "$image" 2>/dev/null || echo "none")

# 3. Перезапускаем только если что-то действительно поменялось, чтобы таймер не
#    дёргал бота каждые полчаса и не рвал пользователям сессии на ровном месте.
if [ "$before_image" = "$after_image" ] && [ "$before_commit" = "$after_commit" ]; then
  log "обновлений нет"
  exit 0
fi

log "применяю обновление"
docker compose up -d

# Старые образы копятся и съедают диск - на маленьком VPS это важно.
docker image prune -f >/dev/null

log "готово, версия: $(docker compose logs --tail 200 bot 2>/dev/null | grep -o 'tgbotkpfu [^ ]* starting' | tail -1 || echo 'см. docker compose logs')"
