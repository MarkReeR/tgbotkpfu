from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from app.api.routes import router, web_router

api_app = FastAPI()
api_app.include_router(router, prefix="/api")
api_app.include_router(web_router)
api_app.mount("/static", StaticFiles(directory="app/api/static"), name="static")