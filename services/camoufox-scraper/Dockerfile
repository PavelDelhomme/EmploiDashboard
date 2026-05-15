FROM python:3.12-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    libgtk-3-0 libdbus-glib-1-2 libxt6 libasound2 libx11-xcb1 \
    libxcomposite1 libxdamage1 libxfixes3 libdrm2 libgbm1 \
    libxrandr2 libcairo2 libpango-1.0-0 libatk1.0-0 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt \
    && python -m camoufox fetch

COPY app.py .

EXPOSE 8765
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8765"]
