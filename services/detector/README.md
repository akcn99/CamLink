# CamLink Detector Service

车辆驶入检测的独立服务（Phase 3）。

## 功能

- 从 CamLink 拉取检测配置
- 连接 RTSP 拉流并做车辆检测
- 判断“驶入区域”事件
- 写入 CamLink 事件库
- Telegram 推送截图和消息

## 必要环境变量

- `CAMLINK_API_BASE`：CamLink API 地址，默认 `http://camlink-app:8083`
- `CAMLINK_DETECTOR_TOKEN`：CamLink 检测访问 Token（来自 `server.detection.access_token`）
- `TELEGRAM_BOT_TOKEN`：Telegram Bot Token（可选）

## 模型参数

- `YOLO_MODEL_PATH`：模型存放路径（默认 `/data/models/yolov5n.onnx`）
- `YOLO_MODEL_URL`：模型下载地址（默认 YOLOv5n 官方 ONNX）
- `YOLO_INPUT_SIZE`：输入尺寸（默认 640）
- `YOLO_CONF`：置信度阈值（默认 0.35）
- `YOLO_IOU`：NMS 阈值（默认 0.45）

## 本地单独运行

```bash
docker build -t camlink-detector:dev services/detector

docker run --rm -p 8091:8091 \
  -e CAMLINK_API_BASE=http://127.0.0.1:8083 \
  -e CAMLINK_DETECTOR_TOKEN=YOUR_TOKEN \
  -e TELEGRAM_BOT_TOKEN=YOUR_BOT_TOKEN \
  -v $(pwd)/save:/data \
  camlink-detector:dev
```

健康检查：

```bash
curl http://127.0.0.1:8091/healthz
```
