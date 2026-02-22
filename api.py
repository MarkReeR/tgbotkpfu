import uvicorn
import logging
from logging.handlers import RotatingFileHandler
from app.services.config import cfg
from app.api import api_app
from app.services.csv_cache import ensure_startup_cache
import asyncio
from app import main


logger = logging.getLogger(__name__)

if __name__ == "__main__":
    main.setup_logging()
    logger.info("Запуск api...")
    asyncio.run(ensure_startup_cache())
    uvicorn.run(api_app, host="127.0.0.1", port=5000, log_config=None)