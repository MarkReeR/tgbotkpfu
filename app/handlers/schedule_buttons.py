import logging
from datetime import datetime, timedelta, date
from typing import Dict, List, Tuple
import re
from aiogram import Router, types
from aiogram.utils.keyboard import ReplyKeyboardBuilder

router = Router()
logger = logging.getLogger(__name__)

user_data: Dict[int, Tuple[str, List[dict]]] = {}


def get_schedule_keyboard():
    builder = ReplyKeyboardBuilder()
    builder.add(types.KeyboardButton(text="📅 Сегодня"))
    builder.add(types.KeyboardButton(text="📅 Завтра"))
    builder.add(types.KeyboardButton(text="📋 Вся неделя"))
    builder.add(types.KeyboardButton(text="🔍 Другая группа"))
    builder.adjust(3, 1)
    return builder.as_markup(resize_keyboard=True)


def get_week_menu_keyboard():
    builder = ReplyKeyboardBuilder()
    builder.add(types.KeyboardButton(text="🔎 Текущая неделя"))
    builder.add(types.KeyboardButton(text="➡️ Следующая неделя"))
    builder.add(types.KeyboardButton(text="📚 Вся без фильтров"))
    builder.add(types.KeyboardButton(text="⬅️ Назад"))
    builder.adjust(3, 1)
    return builder.as_markup(resize_keyboard=True)


def get_day_name(day_offset: int = 0):
    days = ["Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота", "Воскресенье"]
    today = datetime.now() + timedelta(days=day_offset)
    return days[today.weekday()]


def filter_lessons_by_day(lessons: List[dict], day_name: str):
    return [lesson for lesson in lessons if lesson.get("day") == day_name]


def _time_to_minutes(time_str: str | float | None) -> int:
    if not time_str or not isinstance(time_str, str):
        time_str = str(time_str or "").strip()
    if not time_str or time_str.lower() in ("nan", "none", ""):
        return 0
    try:
        hours, minutes = map(int, time_str.split(":"))
        return hours * 60 + minutes
    except (ValueError, AttributeError):
        return 0


def get_current_week_type(start_date: date = date(2025, 9, 1),target_date: date | None = None,):
    d = target_date or date.today()
    weeks_passed = (d - start_date).days // 7
    return "в" if weeks_passed % 2 == 0 else "н"


def _norm_week(x: str) -> str:
    x = (x or "").strip().lower()
    return "в" if x.startswith("в") else ("н" if x.startswith("н") else x)


def filter_by_week(lessons: list[dict], target_date: date | None = None) -> list[dict]:
    wt = get_current_week_type(target_date=target_date)
    return [l for l in lessons if not l.get("week_type") or _norm_week(l.get("week_type")) == wt]


def format_day_schedule(lessons: List[dict], day_name: str, show_week_per_lesson: bool = False):
    if not lessons:
        return f"<b>{day_name}</b>\n\nЗанятий нет\n"

    lessons.sort(key=lambda l: _time_to_minutes(l.get("time", "")))

    if show_week_per_lesson:
        header = f"<b>{day_name}</b>"
    else:
        week = (lessons[0].get("week_type") or "").strip()
        header = f"<b>{day_name} [{week}]</b>" if week else f"<b>{day_name}</b>"

    sep = "—" * 20
    out = [header, sep]
    def _safe_str(x) -> str:
        if x is None:
            return ""
        s = str(x).strip()
        return "" if s.lower() in ("nan", "none", "", "null") else s

    for les in lessons:
        time = _safe_str(les.get("time", ""))
        week = _safe_str(les.get("week_type", ""))
        subj = _safe_str(les.get("subject", ""))
        ltype = _safe_str(les.get("type", ""))
        building = _safe_str(les.get("building", ""))
        room1 = _safe_str(les.get("room1", ""))
        room2 = _safe_str(les.get("room2", ""))
        traw = _safe_str(les.get("teacher", ""))

        line_time = f"⏰ {time}" + (f" [{week}]" if show_week_per_lesson and week else "")
        line_subject = f"{subj}" if subj else ""
        line_type = f"({ltype})" if ltype else ""

        # BUG
        rooms = ", ".join([x for x in (room1, room2) if x])
        loc_parts = ([building] if building else []) + ([f"<i>ауд. {rooms}</i>"] if rooms else [])
        loc = ", ".join(loc_parts)

        clean_t = re.sub(r'[\u00A0\u2000-\u200B]', ' ', traw)
        teachers = [s.strip() for s in re.split(r'[;,]|\s{2,}|\t+', clean_t) if s.strip()]
        teach = ", ".join(teachers)

        line_place = " — ".join([loc, teach]) if (loc and teach) else (loc or teach)

        block = "\n".join(x for x in (line_time if time else "", line_subject, line_type, line_place) if x)
        out.append(block)
        out.append(sep)

    return "\n".join(out)


@router.message(lambda m: m.text in [
    "📅 Сегодня", "📅 Завтра", "📋 Вся неделя", "🔍 Другая группа",
    "🔎 Текущая неделя", "➡️ Следующая неделя", "📚 Вся без фильтров", "⬅️ Назад"
])
async def handle_schedule_buttons(message: types.Message) -> None:
    user_id = message.from_user.id

    if message.text == "🔍 Другая группа":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        await message.answer(
            "Введите номер группы:\nПример: 09-825, 8251160, 8251",
            reply_markup=types.ReplyKeyboardRemove(),
        )
        return

    if message.text == "⬅️ Назад":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        await message.answer("Выберите действие:", reply_markup=get_schedule_keyboard())
        return

    if user_id not in user_data:
        await message.answer(
            "❌ Расписание не найдено. Сначала найдите группу:",
            reply_markup=types.ReplyKeyboardRemove(),
        )
        return

    group, lessons = user_data[user_id]

    if message.text == "📅 Сегодня":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        day_name = get_day_name(0)
        day_lessons = filter_lessons_by_day(lessons, day_name)
        day_lessons = filter_by_week(day_lessons, target_date=date.today())
        await message.answer(
            format_day_schedule(day_lessons, day_name),
            parse_mode="HTML", disable_web_page_preview=True
        )

    elif message.text == "📅 Завтра":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        day_name = get_day_name(1)
        day_lessons = filter_lessons_by_day(lessons, day_name)
        day_lessons = filter_by_week(day_lessons, target_date=date.today() + timedelta(days=1))
        await message.answer(
            format_day_schedule(day_lessons, day_name),
            parse_mode="HTML", disable_web_page_preview=True
        )

    elif message.text == "📋 Вся неделя":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        await message.answer("Выберите фильтр:", reply_markup=get_week_menu_keyboard())

    elif message.text == "🔎 Текущая неделя":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        await message.answer(
            f"📆 <b>Расписание на текущую неделю</b>\nГруппа: <b>{group}</b>",
            parse_mode="HTML", reply_markup=get_week_menu_keyboard()
        )
        days_order = ["Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"]
        for day in days_order:
            day_lessons = filter_lessons_by_day(lessons, day)
            day_lessons = filter_by_week(day_lessons, target_date=date.today())
            day_text = format_day_schedule(day_lessons, day)
            await message.answer(day_text, parse_mode="HTML", disable_web_page_preview=True)

    elif message.text == "➡️ Следующая неделя":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        await message.answer(
            f"📆 <b>Расписание на следующую неделю</b>\nГруппа: <b>{group}</b>",
            parse_mode="HTML", reply_markup=get_week_menu_keyboard()
        )
        days_order = ["Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"]
        target = date.today() + timedelta(days=7)
        for day in days_order:
            day_lessons = filter_lessons_by_day(lessons, day)
            day_lessons = filter_by_week(day_lessons, target_date=target)
            day_text = day_text = format_day_schedule(day_lessons, day)
            await message.answer(day_text, parse_mode="HTML", disable_web_page_preview=True)

    elif message.text == "📚 Вся без фильтров":
        logger.info("Пользователь %s: %s", message.from_user.id, message.text)
        await message.answer(
            f"📆 <b>Расписание на неделю (без фильтра)</b>\nГруппа: <b>{group}</b>",
            parse_mode="HTML", reply_markup=get_week_menu_keyboard()
        )
        days_order = ["Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"]
        for day in days_order:
            day_lessons = filter_lessons_by_day(lessons, day)
            day_text = format_day_schedule(day_lessons, day, show_week_per_lesson=True)
            await message.answer(day_text, parse_mode="HTML", disable_web_page_preview=True)
