# CamLink Detector Service

这是车辆驶入检测的独立服务骨架，当前处于 Phase 1。

当前仅提供：
- `GET /healthz`
- `GET /v1/detector/info`

后续阶段会逐步加入：
- RTSP 取帧
- 车辆检测
- 区域进入判定
- Telegram 推送
- 事件写库

## 本地单独运行

```bash
docker build -t camlink-detector:dev services/detector
docker run --rm -p 8091:8091 camlink-detector:dev
```

健康检查：

```bash
curl http://127.0.0.1:8091/healthz
```
