import html
import logging
from aiogram import Router, types
from aiogram.filters import Command

from app.services.config import cfg
from app.handlers.schedule_buttons import get_schedule_keyboard, user_data
from app.services.csv_cache import find_group_schedule_local
from app.services.parser import parse_schedule

router = Router()
logger = logging.getLogger(__name__)


def _norm_group(s: str) -> str:
    return "".join(ch for ch in (s or "") if ch.isdigit())


@router.message(Command("schedule"))
@router.message(lambda message: message.text and not message.text.startswith("/"))
async def cmd_schedule(message: types.Message) -> None:
    logger.info("Пользователь %s запросил расписание: %s", message.from_user.id, message.text)

    if message.text.startswith("/"):
        args = (message.text or "").split(maxsplit=1)[1:]
        if not args:
            await message.answer("Использование: /schedule <Группа>\nПример: /schedule 09-825")
            return
        group_input = args[0]
    else:
        group_input = message.text.strip()

    group = _norm_group(group_input)
    if not group:
        await message.answer(
            "Не распознал номер группы. Пример: 8251160\n"
            "Попробуйте еще раз:"
        )
        return

    if len(group) != 7:
        await message.answer(
            "❗Номер группы должен содержать ровно 7 цифр.\n"
            "Попробуйте снова:"
        )
        return

    status_msg = await message.answer(f"🔍 Ищу группу {group}...")

    csv_text = find_group_schedule_local(group)
    if not csv_text:
        await status_msg.edit_text(
            f"❌ Группа <b>{html.escape(group)}</b> не найдена.\n"
            "Проверьте правильность написания номера группы.",
            parse_mode="HTML",
        )
        return

    try:
        lessons = parse_schedule(csv_text, group)
    except Exception as e:
        logger.exception("Ошибка парсинга для группы %s", group)
        await status_msg.edit_text(
            f"▲ Ошибка обработки: <code>{html.escape(str(e))}</code>",
            parse_mode="HTML",
        )
        return

    user_data[message.from_user.id] = (group, lessons)

    if not lessons:
        await status_msg.edit_text(
            f"ℹ️ Группа <b>{html.escape(group)}</b> найдена, но расписание пустое.\n"
            "Возможно, на этой неделе нет занятий.",
            parse_mode="HTML",
        )
        return

    await status_msg.delete()

    await message.answer(
        f"✅ Группа <b>{html.escape(group)}</b> найдена!\n"
        "Выберите период для просмотра:",
        parse_mode="HTML",
        reply_markup=get_schedule_keyboard(),
    )
