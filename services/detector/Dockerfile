FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

RUN apt-get update \
    && apt-get install -y --no-install-recommends tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY app.py ./

ENV CAMLINK_DETECTOR_PORT=8091
ENV TZ=Asia/Shanghai
EXPOSE 8091

CMD ["python", "app.py"]
