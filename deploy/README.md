# Автообновление на сервере

Таймер systemd раз в полчаса проверяет, не появилась ли новая версия образа в
GHCR, и перезапускает контейнер только если она действительно появилась.
Заходить на сервер после каждого изменения больше не нужно.

## Установка

Из каталога с ботом (`~/tgbotkpfu`):

```bash
chmod +x deploy/update.sh
```

Проверить, что скрипт работает, до того как отдавать его systemd:

```bash
./deploy/update.sh
```

Если пользователь или путь отличаются от `markreer` и `/home/markreer/tgbotkpfu`,
поправьте их в `deploy/tgbotkpfu-update.service`, затем:

```bash
sudo cp deploy/tgbotkpfu-update.service deploy/tgbotkpfu-update.timer /etc/systemd/system/
```

```bash
sudo systemctl daemon-reload && sudo systemctl enable --now tgbotkpfu-update.timer
```

## Проверка

Когда сработает в следующий раз:

```bash
systemctl list-timers tgbotkpfu-update.timer
```

Что делал в прошлый раз:

```bash
journalctl -u tgbotkpfu-update.service -n 30 --no-pager
```

Запустить проверку немедленно, не дожидаясь таймера:

```bash
sudo systemctl start tgbotkpfu-update.service
```

## Как это работает

1. `git fetch` + `git reset --hard` на текущую ветку — подтягивает в том числе
   изменения самого `docker-compose.yml`.
2. `docker compose pull` — скачивает образ, если тег `latest` переехал.
3. Сравнивает ID образа и хеш коммита до и после. Если оба не изменились —
   выходит, ничего не трогая. Это важно: иначе таймер перезапускал бы бота
   каждые полчаса просто так.
4. `docker compose up -d` и `docker image prune -f`, чтобы старые образы не
   копились на диске.

## Важно про локальные правки

Скрипт делает `git reset --hard`, то есть **любые изменения файлов репозитория
на сервере будут затёрты**. Это сознательно: сервер должен быть точной копией
`main`, иначе обновления начнут конфликтовать.

Файл `.env` при этом в безопасности — он в `.gitignore` и `reset` его не
касается. Все настройки держите там, а не в отслеживаемых файлах.

## Отключить

```bash
sudo systemctl disable --now tgbotkpfu-update.timer
```
