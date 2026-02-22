from pydantic import BaseModel, Field
from typing import List, Optional
from datetime import date


class Group(BaseModel):
    number: str = Field(..., title="Group number", max_length=7)

class ScheduleFull(BaseModel):
    data: list = Field(..., title="Full data")
    status_code: int = Field(..., title="Status code")

class Date(BaseModel):
    day: int = Field(..., title="day")
    month: int = Field(..., title="month")
    year: int = Field(..., title="year")

class WeekType(BaseModel):
    week_type: str = Field(..., title="week type v or n")
    status_code: int = Field(..., title="Status code")


class AdminLogin(BaseModel):
    username: str = Field(..., min_length=1, max_length=50)
    password: str = Field(..., min_length=1, max_length=100)