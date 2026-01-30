import logging
from io import StringIO
from typing import List, Dict
import re

import pandas as pd

logger = logging.getLogger(__name__)


def _clean_series(series: pd.Series):
    return series.astype(str).str.strip().replace("nan", "")


def _strip_dot_zero(series: pd.Series):
    return series.apply(
        lambda s: re.sub(r'^(\d+)\.0$', r'\1', s) if isinstance(s, str) else s
    )


def parse_schedule(csv_text: str, group_code: str) -> List[Dict]:
    if not csv_text:
        logger.warning("Получен пустой CSV для группы %s", group_code)
        return []

    try:
        df = pd.read_csv(StringIO(csv_text), header=[0, 1])
    except Exception as e:
        logger.error("Ошибка при чтении CSV для группы %s: %s", group_code, e)
        return []

    col_day, col_time, col_week = df.columns[:3]
    days = df[col_day].ffill()
    times = _clean_series(df[col_time])
    weeks = _clean_series(df[col_week])

    start_idx = next(
        (i for i, col in enumerate(df.columns) if group_code in str(col[0])),
        None,
    )
    if start_idx is None:
        logger.warning("Группа %s не найдена в таблице", group_code)
        return []

    try:
        col_subj = df.columns[start_idx]
        col_build = df.columns[start_idx + 1]
        col_room1 = df.columns[start_idx + 2]
        col_room2 = df.columns[start_idx + 3]
        col_type = df.columns[start_idx + 4]
        col_teacher = df.columns[start_idx + 7]
    except Exception as e:
        logger.error("Ошибка при определении колонок для группы %s: %s", group_code, e)
        return []

    subj = _clean_series(df[col_subj])
    build = _clean_series(df[col_build])
    room1 = _strip_dot_zero(_clean_series(df[col_room1]))
    room2 = _strip_dot_zero(_clean_series(df[col_room2]))
    type_ = _clean_series(df[col_type])
    teach = _clean_series(df[col_teacher])

    out = []
    for d, t, w, s, b, r1, r2, ty, te in zip(
            days, times, weeks, subj, build, room1, room2, type_, teach
    ):
        if s and t:
            out.append(
                {
                    "group": group_code,
                    "day": d,
                    "time": t,
                    "week_type": w,
                    "subject": s,
                    "building": b,
                    "room1": r1,
                    "room2": r2,
                    "type": ty,
                    "teacher": te,
                }
            )

    return out
