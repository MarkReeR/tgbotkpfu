import fastapi
from fastapi import status, HTTPException
from app.services.csv_cache import find_group_schedule_local
from app.services.parser import parse_schedule
from app.services.csv_cache import ensure_startup_cache
from fastapi.responses import JSONResponse
import asyncio
from app.api.utils import generate_valid_schedule_json, get_current_week_type
import app.api.models as models
import datetime
from fastapi.templating import Jinja2Templates
from fastapi import Request
import hashlib
from app.services.config import cfg
import os


router = fastapi.APIRouter()
web_router = fastapi.APIRouter()

templates = Jinja2Templates(directory="app/api/templates")

ADMIN_CREDENTIALS = {
    cfg.api_admin_login: hashlib.sha256(cfg.api_admin_password.encode()).hexdigest()
}

def verify_token(token: str) -> bool:
    if not token:
        return False
    try:
        data = hashlib.sha256(token.encode()).hexdigest()
        return data in ADMIN_CREDENTIALS.values()
    except:
        return False

@router.post(
    "/schedule/full/",
    response_model=models.ScheduleFull,
    status_code=status.HTTP_201_CREATED,
    description="Get full schedule for <number> group (both week)"
)
async def get_group(item: models.Group):
    data = find_group_schedule_local(group_code=item.number)
    if data:
        data = parse_schedule(data, item.number)
        ret = models.ScheduleFull(data=generate_valid_schedule_json(data), status_code=200)


        return ret
    else:
        return JSONResponse(content={"status": f"Invalid group code {item.number}", "status_code": 404}, status_code=status.HTTP_404_NOT_FOUND,
                        headers={"X-My-Header": "value"})


@router.get(
    "/schedule/today-week-type/",
    response_model=models.WeekType,
    status_code=status.HTTP_201_CREATED,
    description="Get today week type (v or n)"
)
async def get_today_week_type():
    return models.WeekType(week_type=get_current_week_type(target_date=datetime.datetime.today().date()), status_code=200)


@web_router.get("/admin")
async def admin_redirect():
    from fastapi.responses import RedirectResponse
    return RedirectResponse(url="/admin/login")


@web_router.get("/admin/login")
async def admin_login_page(request: Request):
    return templates.TemplateResponse("login.html", {"request": request})


@web_router.get("/admin/dashboard")
async def admin_dashboard(request: Request):
    return templates.TemplateResponse("dashboard.html", {"request": request})


@router.post("/admin/login")
async def admin_login(item: models.AdminLogin):
    username = item.username
    password_hash = hashlib.sha256(item.password.encode()).hexdigest()
    
    if username in ADMIN_CREDENTIALS and ADMIN_CREDENTIALS[username] == password_hash:
        token = hashlib.sha256(f"{username}:{password_hash}".encode()).hexdigest()
        return {"token": token, "status_code": 200}
    
    raise HTTPException(status_code=401, detail="Неверный логин или пароль")