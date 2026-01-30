import asyncio
import logging
from logging.handlers import RotatingFileHandler
from datetime import datetime, time, timedelta
from zoneinfo import ZoneInfo

from app.middlewares.antiflood import AntiFloodMiddleware
from app.middlewares.singleflight import SingleFlightMiddleware
from app.services.config import cfg
from app.handlers import start, schedule, schedule_buttons
from app.services.csv_cache import ensure_startup_cache, refresh_all
from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode

logger = logging.getLogger(__name__)


def setup_logging():
    import os
    os.makedirs("logs", exist_ok=True)
    handler = RotatingFileHandler(cfg.log_file, maxBytes=1_000_000, backupCount=3, encoding="utf-8")
    fmt = logging.Formatter("%(asctime)s | %(levelname)-8s | %(name)s | %(message)s")
    handler.setFormatter(fmt)
    root = logging.getLogger()
    root.setLevel(getattr(logging, cfg.log_level.upper(), logging.INFO))
    root.addHandler(handler)


async def _seconds_until_next_run() -> float:
    tz = ZoneInfo(cfg.tz)
    now = datetime.now(tz)
    targets = []
    for hhmm in cfg.refresh_at:
        h, m = map(int, hhmm.split(":"))
        targets.append(datetime.combine(now.date(), time(h, m), tzinfo=tz))
        targets.append(datetime.combine(now.date() + timedelta(days=1), time(h, m), tzinfo=tz))
    future = min(t for t in targets if t > now)
    return (future - now).total_seconds()


async def _cron_refresh_task():
    while True:
        try:
            secs = await _seconds_until_next_run()
            logger.info("Следующее обновление CSV через %.0f сек.", secs)
            await asyncio.sleep(secs)
            await refresh_all()
        except asyncio.CancelledError:
            raise
        except Exception as e:
            logger.exception("Ошибка в планировщике обновлений: %s", e)
            await asyncio.sleep(60)


async def main() -> None:
    setup_logging()
    logger.info("Запуск бота...")

    await ensure_startup_cache()

    asyncio.create_task(_cron_refresh_task())

    bot = Bot(
        token=cfg.bot_token,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML),
    )
    dp = Dispatcher()

    dp.message.middleware(SingleFlightMiddleware())
    dp.callback_query.middleware(SingleFlightMiddleware())

    dp.message.middleware(AntiFloodMiddleware(cooldown_sec=1.2))
    dp.callback_query.middleware(AntiFloodMiddleware(cooldown_sec=0.7))

    dp.include_router(start.router)
    dp.include_router(schedule_buttons.router)
    dp.include_router(schedule.router)

    try:
        await dp.start_polling(bot)
    except asyncio.CancelledError:
        logger.warning("Polling остановлен (CancelledError)")
    finally:
        await bot.session.close()
        logger.info("Бот завершил работу.")
