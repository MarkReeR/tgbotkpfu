import asyncio
import logging
import os
import re
from pathlib import Path
from typing import List, Optional, Dict

from app.services.config import cfg
from app.services.google_csv import fetch_csv_text

logger = logging.getLogger(__name__)

GROUP_INDEX: Dict[str, str] = {} # group_code: gid_id,csv

def _cache_dir():
    d = Path(os.getenv("CACHE_DIR", getattr(cfg, "cache_dir", "data/csv")))
    d.mkdir(parents=True, exist_ok=True)
    return d


def _gid_path(gid: int):
    return _cache_dir() / f"gid_{gid}.csv"


def list_cached_files():
    d = _cache_dir()
    return sorted([p for p in d.glob("gid_*.csv") if p.is_file()])


async def download_gid(gid: int):
    csv_text = await fetch_csv_text(cfg.spreadsheet_id, gid)
    if not csv_text:
        logger.warning("Не удалось скачать CSV для GID=%s", gid)
        return None

    tmp = _gid_path(gid).with_suffix(".csv.tmp")
    tmp.write_text(csv_text, encoding="utf-8")
    tmp.replace(_gid_path(gid))
    logger.info("CSV сохранён: %s", _gid_path(gid))
    return _gid_path(gid)


async def download_all(gids: Optional[List[int]] = None):
    gids = gids or cfg.gids
    sem = asyncio.Semaphore(4)
    saved: List[Path] = []

    async def _one(g):
        async with sem:
            p = await download_gid(g)
            if p:
                saved.append(p)

    await asyncio.gather(*[_one(g) for g in gids])
    return saved


async def ensure_startup_cache():
    existing = {int(p.stem.split("_")[1]) for p in list_cached_files()}
    missing = [g for g in cfg.gids if g not in existing]

    if not existing:
        logger.info("Кэш пуст — первичная загрузка CSV (%d листов)...", len(cfg.gids))
        await download_all()
    elif missing:
        logger.info("В кэше отсутствуют %d лист(ов): %s — докачиваю.", len(missing), missing)
        await download_all(missing)
    else:
        logger.info("CSV уже есть в кэше (%d файлов).", len(existing))
    
    _build_group_index()


async def refresh_all():
    logger.info("Обновление CSV: скачиваю новые версии и заменяю старые...")
    saved = await download_all()
    _build_group_index()
    logger.info("Готово. Обновлено файлов: %d", len(saved))
    


def _build_group_index():
    """Сканирует все CSV и строит индекс групп."""
    global GROUP_INDEX
    GROUP_INDEX.clear()
    
    for path in list_cached_files():
        try:
            with open(path, "r", encoding="utf-8") as f:
                header = f.readline()
            
            # groups = re.findall(r'\b\d{7}\b', header)
            all_digits = re.findall(r'\d+', header)
            groups = [digits for digits in all_digits if len(digits) == 7]
            logger.debug("Найдены группы в %s: %s", path.name, groups)
            for group in groups:
                if group not in GROUP_INDEX:
                    GROUP_INDEX[group] = path.name
                    
        except Exception as e:
            logger.warning("Не удалось проиндексировать %s: %s", path, e)
    
    logger.info("Построен индекс для %d групп", len(GROUP_INDEX))


def find_group_schedule_local(group_code: str):
    clean_code = "".join(ch for ch in (group_code or "") if ch.isdigit())
    if len(clean_code) != 7:
        return None
    
    filename = GROUP_INDEX.get(clean_code)
    if not filename:
        logger.warning("Группа %s не найдена в индексе", clean_code)
        return None
    
    path = _cache_dir() / filename
    try:
        return path.read_text(encoding="utf-8")
    except Exception as e:
        logger.error("Ошибка чтения %s: %s", path, e)
        return None