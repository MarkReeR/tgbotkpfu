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
# `| head -1` под `set -o pipefail` может уронить скрипт: head закрывает пайп,
# docker получает SIGPIPE и возвращает ненулевой код. Поэтому режем строку
# средствами шелла, без пайпа.
images=$(docker compose config --images)
image=${images%%$'\n'*}
if [ -z "$image" ]; then
  log "не удалось определить образ из docker-compose.yml"
  exit 1
fi

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

# Даём контейнеру пару секунд подняться, потом достаём строку с версией.
# pipefail здесь мешает (grep без совпадений возвращает 1), поэтому отключаем
# его на время этой проверки - её неудача не повод считать деплой провальным.
sleep 3
set +o pipefail
version=$(docker compose logs --tail 200 bot 2>/dev/null | grep -o 'tgbotkpfu [^ ]* starting' | tail -1)
set -o pipefail

log "готово${version:+, }${version:-, версию см. в docker compose logs}"

# Если контейнер не поднялся - об этом надо знать сразу, а не от пользователей.
if [ -z "$(docker compose ps --quiet --status running)" ]; then
  log "ВНИМАНИЕ: контейнер не запущен, смотрите docker compose logs"
  exit 1
fi
