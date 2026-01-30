import time
from typing import Any, Awaitable, Callable, Dict, Optional

from aiogram import BaseMiddleware, types


class AntiFloodMiddleware(BaseMiddleware):
    def __init__(self, cooldown_sec: float = 1.5):
        self.cooldown = cooldown_sec
        self._last_by_user: Dict[int, float] = {}

    async def __call__(
            self,
            handler: Callable[[types.TelegramObject, Dict[str, Any]], Awaitable[Any]],
            event: types.TelegramObject,
            data: Dict[str, Any]
    ) -> Any:
        user_id: Optional[int] = getattr(getattr(event, "from_user", None), "id", None)
        if not user_id:
            return await handler(event, data)

        now = time.monotonic()
        last = self._last_by_user.get(user_id, 0.0)
        if now - last < self.cooldown:
            if isinstance(event, types.Message):
                await event.answer("⏳ Пожалуйста, не нажимайте так часто.")
            elif isinstance(event, types.CallbackQuery):
                await event.answer("⏳ Подождите…", show_alert=False)
            return
        self._last_by_user[user_id] = now
        return await handler(event, data)
