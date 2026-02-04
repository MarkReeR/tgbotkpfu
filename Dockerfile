FROM python:3.11-slim AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    && rm -rf /var/lib/apt/lists/*

RUN pip install uv

WORKDIR /app

COPY requirements.txt .
RUN uv pip install --system --compile-bytecode -r requirements.txt

FROM python:3.11-slim

RUN useradd --create-home --shell /bin/bash app
USER app
WORKDIR /home/app

COPY --from=builder --chown=app:app /usr/local /usr/local

COPY --chown=app:app . .

RUN mkdir -p /home/app/logs /home/app/data/csv

# Экспонируем порт для health-check
EXPOSE 8000

CMD ["python", "app/main.py"]