import asyncio
from typing import Any, Awaitable, Callable, Dict

from aiogram import BaseMiddleware, types


class SingleFlightMiddleware(BaseMiddleware):
    def __init__(self):
        self._locks: Dict[int, asyncio.Lock] = {}

    def _get_lock(self, user_id: int):
        lock = self._locks.get(user_id)
        if lock is None:
            lock = asyncio.Lock()
            self._locks[user_id] = lock
        return lock

    async def __call__(
            self,
            handler: Callable[[types.TelegramObject, Dict[str, Any]], Awaitable[Any]],
            event: types.TelegramObject,
            data: Dict[str, Any]
    ) -> Any:
        user = getattr(event, "from_user", None)
        if not user:
            return await handler(event, data)

        lock = self._get_lock(user.id)
        if lock.locked():
            if isinstance(event, types.Message):
                await event.answer("⏳ Обрабатываю предыдущий запрос…")
            elif isinstance(event, types.CallbackQuery):
                await event.answer("⏳ Обрабатываю предыдущий запрос…", show_alert=False)
            return

        async with lock:
            return await handler(event, data)
