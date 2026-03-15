import json
import math
import os
import threading
import time
import uuid
from collections import deque
from datetime import datetime
from dataclasses import dataclass
from typing import Dict, List, Optional, Tuple

import cv2
import numpy as np
import requests
from flask import Flask, jsonify, request, Response

app = Flask(__name__)

PORT = int(os.getenv("CAMLINK_DETECTOR_PORT", "8091"))
CAMLINK_API_BASE = os.getenv("CAMLINK_API_BASE", "http://camlink-app:8083").rstrip("/")
CAMLINK_DETECTOR_TOKEN = os.getenv("CAMLINK_DETECTOR_TOKEN", "").strip()
POLL_INTERVAL = float(os.getenv("CAMLINK_POLL_INTERVAL", "5"))

MODEL_VARIANT = os.getenv("YOLO_MODEL_VARIANT", "yolov5n").strip().lower()
MODEL_PATH = os.getenv("YOLO_MODEL_PATH", "").strip()
MODEL_URL = os.getenv("YOLO_MODEL_URL", "").strip()
MODEL_VARIANT_URLS = {
    "yolov5n": "https://github.com/ultralytics/yolov5/releases/download/v7.0/yolov5n.onnx",
    "yolov5s": "https://github.com/ultralytics/yolov5/releases/download/v7.0/yolov5s.onnx",
}
if not MODEL_VARIANT or MODEL_VARIANT not in MODEL_VARIANT_URLS:
    MODEL_VARIANT = "yolov5n"
if not MODEL_PATH:
    MODEL_PATH = f"/data/models/{MODEL_VARIANT}.onnx"
if not MODEL_URL:
    MODEL_URL = MODEL_VARIANT_URLS.get(MODEL_VARIANT, MODEL_VARIANT_URLS["yolov5n"])
MODEL_INPUT = int(os.getenv("YOLO_INPUT_SIZE", "640"))
CONF_THRESHOLD = float(os.getenv("YOLO_CONF", "0.35"))
IOU_THRESHOLD = float(os.getenv("YOLO_IOU", "0.45"))
RTSP_READ_TIMEOUT_SEC = float(os.getenv("CAMLINK_RTSP_READ_TIMEOUT", "10"))
RTSP_OPEN_TIMEOUT_SEC = float(os.getenv("CAMLINK_RTSP_OPEN_TIMEOUT", "10"))
RTSP_TRANSPORT = os.getenv("CAMLINK_RTSP_TRANSPORT", "tcp").strip().lower()
RTSP_MAX_CONSEC_FAILS = int(os.getenv("CAMLINK_RTSP_MAX_FAILS", "5"))
RTSP_BACKOFF_BASE_SEC = float(os.getenv("CAMLINK_RTSP_BACKOFF_BASE", "0.5"))
RTSP_BACKOFF_MAX_SEC = float(os.getenv("CAMLINK_RTSP_BACKOFF_MAX", "5"))

SNAPSHOT_DIR = os.getenv("SNAPSHOT_DIR", "/data/snapshots")

COCO_CLASSES = [
    "person",
    "bicycle",
    "car",
    "motorcycle",
    "airplane",
    "bus",
    "train",
    "truck",
    "boat",
    "traffic light",
    "fire hydrant",
    "stop sign",
    "parking meter",
    "bench",
    "bird",
    "cat",
    "dog",
    "horse",
    "sheep",
    "cow",
    "elephant",
    "bear",
    "zebra",
    "giraffe",
    "backpack",
    "umbrella",
    "handbag",
    "tie",
    "suitcase",
    "frisbee",
    "skis",
    "snowboard",
    "sports ball",
    "kite",
    "baseball bat",
    "baseball glove",
    "skateboard",
    "surfboard",
    "tennis racket",
    "bottle",
    "wine glass",
    "cup",
    "fork",
    "knife",
    "spoon",
    "bowl",
    "banana",
    "apple",
    "sandwich",
    "orange",
    "broccoli",
    "carrot",
    "hot dog",
    "pizza",
    "donut",
    "cake",
    "chair",
    "couch",
    "potted plant",
    "bed",
    "dining table",
    "toilet",
    "tv",
    "laptop",
    "mouse",
    "remote",
    "keyboard",
    "cell phone",
    "microwave",
    "oven",
    "toaster",
    "sink",
    "refrigerator",
    "book",
    "clock",
    "vase",
    "scissors",
    "teddy bear",
    "hair drier",
    "toothbrush",
]

DEFAULT_VEHICLE_CLASSES = {"car", "motorcycle", "bicycle"}


@dataclass
class DetectionConfig:
    stream_uuid: str
    stream_name: str
    channel_id: str
    stream_url: str
    mode: str
    sample_fps: int
    cooldown_seconds: int
    confidence_threshold: float
    min_box_area: int
    min_move_px: int
    entry_direction: str
    entry_line: List[Dict[str, float]]
    classes: List[str]
    polygon: List[Dict[str, float]]
    trigger_consecutive_frames: int
    telegram_enabled: bool
    telegram_chat_id: str


@dataclass
class TrackState:
    track_id: str
    centroid: Tuple[int, int]
    last_seen: float
    object_class: str
    inside: bool = False
    seen_outside: bool = False
    inside_streak: int = 0
    triggered: bool = False
    age: int = 0
    prev_centroid: Optional[Tuple[int, int]] = None
    move_distance: float = 0.0
    first_centroid: Optional[Tuple[int, int]] = None
    last_outside_centroid: Optional[Tuple[int, int]] = None
    box_area: float = 0.0
    first_inside_ts: Optional[float] = None


class SimpleTracker:
    def __init__(self, max_distance: float = 80.0, max_age: int = 5):
        self.max_distance = max_distance
        self.max_age = max_age
        self.tracks: Dict[str, TrackState] = {}

    def update(self, detections: List[Tuple[str, Tuple[int, int], float]]) -> List[TrackState]:
        assigned = set()
        track_ids = list(self.tracks.keys())
        # Age all tracks
        for track in self.tracks.values():
            track.age += 1

        # Build distance matrix
        pairs = []
        for ti, track_id in enumerate(track_ids):
            track = self.tracks[track_id]
            for di, det in enumerate(detections):
                _, centroid, _ = det
                dist = math.hypot(centroid[0] - track.centroid[0], centroid[1] - track.centroid[1])
                pairs.append((dist, track_id, di))
        pairs.sort(key=lambda x: x[0])

        used_dets = set()
        for dist, track_id, det_idx in pairs:
            if dist > self.max_distance:
                continue
            if track_id in assigned or det_idx in used_dets:
                continue
            det_class, centroid, det_area = detections[det_idx]
            track = self.tracks[track_id]
            if track.centroid:
                track.prev_centroid = track.centroid
                track.move_distance = math.hypot(centroid[0] - track.centroid[0], centroid[1] - track.centroid[1])
            track.centroid = centroid
            if track.first_centroid is None:
                track.first_centroid = centroid
            track.last_seen = time.time()
            track.object_class = det_class
            track.box_area = det_area
            track.age = 0
            assigned.add(track_id)
            used_dets.add(det_idx)

        # Add new tracks
        for det_idx, det in enumerate(detections):
            if det_idx in used_dets:
                continue
            det_class, centroid, det_area = det
            track_id = f"trk-{uuid.uuid4().hex[:8]}"
            self.tracks[track_id] = TrackState(
                track_id=track_id,
                centroid=centroid,
                last_seen=time.time(),
                object_class=det_class,
                box_area=det_area,
                first_centroid=centroid,
                last_outside_centroid=centroid,
            )

        # Drop stale tracks
        for track_id in list(self.tracks.keys()):
            if self.tracks[track_id].age > self.max_age:
                del self.tracks[track_id]

        return list(self.tracks.values())


def download_model_if_needed() -> bool:
    if os.path.isfile(MODEL_PATH):
        return True
    os.makedirs(os.path.dirname(MODEL_PATH), exist_ok=True)
    try:
        with requests.get(MODEL_URL, stream=True, timeout=30) as resp:
            resp.raise_for_status()
            with open(MODEL_PATH, "wb") as f:
                for chunk in resp.iter_content(chunk_size=8192):
                    if chunk:
                        f.write(chunk)
        return True
    except Exception as exc:
        app.logger.error("model download failed: %s", exc)
        return False


def letterbox(image: np.ndarray, new_shape: int = 640, color=(114, 114, 114)):
    shape = image.shape[:2]  # height, width
    if isinstance(new_shape, int):
        new_shape = (new_shape, new_shape)
    r = min(new_shape[0] / shape[0], new_shape[1] / shape[1])
    new_unpad = (int(round(shape[1] * r)), int(round(shape[0] * r)))
    dw = new_shape[1] - new_unpad[0]
    dh = new_shape[0] - new_unpad[1]
    dw /= 2
    dh /= 2

    if shape[::-1] != new_unpad:
        image = cv2.resize(image, new_unpad, interpolation=cv2.INTER_LINEAR)
    top, bottom = int(round(dh - 0.1)), int(round(dh + 0.1))
    left, right = int(round(dw - 0.1)), int(round(dw + 0.1))
    image = cv2.copyMakeBorder(image, top, bottom, left, right, cv2.BORDER_CONSTANT, value=color)
    return image, r, (dw, dh)


def point_in_polygon(point: Tuple[int, int], polygon: List[Dict[str, int]]) -> bool:
    if not polygon:
        return False
    x, y = point
    inside = False
    n = len(polygon)
    for i in range(n):
        j = (i - 1) % n
        xi, yi = polygon[i]["x"], polygon[i]["y"]
        xj, yj = polygon[j]["x"], polygon[j]["y"]
        intersect = ((yi > y) != (yj > y)) and (
            x < (xj - xi) * (y - yi) / float(yj - yi + 1e-6) + xi
        )
        if intersect:
            inside = not inside
    return inside


def safe_float(value: object) -> float:
    try:
        return float(value)
    except Exception:
        return 0.0


def normalize_polygon(polygon: List[Dict[str, float]], frame_w: int, frame_h: int) -> List[Dict[str, int]]:
    if not polygon:
        return []
    xs = [safe_float(p.get("x", 0)) for p in polygon]
    ys = [safe_float(p.get("y", 0)) for p in polygon]
    if not xs or not ys:
        return []
    max_x = max(xs)
    max_y = max(ys)
    is_ratio = max_x <= 1.01 and max_y <= 1.01
    points = []
    for x, y in zip(xs, ys):
        if is_ratio:
            px = int(round(x * frame_w))
            py = int(round(y * frame_h))
        else:
            px = int(round(x))
            py = int(round(y))
        px = max(0, min(frame_w - 1, px))
        py = max(0, min(frame_h - 1, py))
        points.append({"x": px, "y": py})
    return points


def normalize_line(line: List[Dict[str, float]], frame_w: int, frame_h: int) -> List[Dict[str, int]]:
    if not line or len(line) != 2:
        return []
    xs = [safe_float(p.get("x", 0)) for p in line]
    ys = [safe_float(p.get("y", 0)) for p in line]
    max_x = max(xs) if xs else 0.0
    max_y = max(ys) if ys else 0.0
    is_ratio = max_x <= 1.01 and max_y <= 1.01
    points = []
    for x, y in zip(xs, ys):
        if is_ratio:
            px = int(round(x * frame_w))
            py = int(round(y * frame_h))
        else:
            px = int(round(x))
            py = int(round(y))
        px = max(0, min(frame_w - 1, px))
        py = max(0, min(frame_h - 1, py))
        points.append({"x": px, "y": py})
    return points


def line_side(point: Tuple[int, int], a: Dict[str, int], b: Dict[str, int]) -> float:
    return (b["x"] - a["x"]) * (point[1] - a["y"]) - (b["y"] - a["y"]) * (point[0] - a["x"])


def point_dist_to_segment(point: Tuple[int, int], a: Dict[str, int], b: Dict[str, int]) -> float:
    ax, ay = float(a["x"]), float(a["y"])
    bx, by = float(b["x"]), float(b["y"])
    px, py = float(point[0]), float(point[1])
    dx = bx - ax
    dy = by - ay
    if dx == 0 and dy == 0:
        return math.hypot(px - ax, py - ay)
    t = ((px - ax) * dx + (py - ay) * dy) / (dx * dx + dy * dy)
    t = max(0.0, min(1.0, t))
    proj_x = ax + t * dx
    proj_y = ay + t * dy
    return math.hypot(px - proj_x, py - proj_y)


def segments_intersect(p1: Tuple[int, int], p2: Tuple[int, int], a: Dict[str, int], b: Dict[str, int], eps: float = 1e-6) -> bool:
    def orient(p, q, r):
        return (q[0] - p[0]) * (r[1] - p[1]) - (q[1] - p[1]) * (r[0] - p[0])

    def on_segment(p, q, r):
        return (
            min(p[0], q[0]) - eps <= r[0] <= max(p[0], q[0]) + eps
            and min(p[1], q[1]) - eps <= r[1] <= max(p[1], q[1]) + eps
        )

    a_pt = (a["x"], a["y"])
    b_pt = (b["x"], b["y"])
    o1 = orient(p1, p2, a_pt)
    o2 = orient(p1, p2, b_pt)
    o3 = orient(a_pt, b_pt, p1)
    o4 = orient(a_pt, b_pt, p2)

    if abs(o1) <= eps and on_segment(p1, p2, a_pt):
        return True
    if abs(o2) <= eps and on_segment(p1, p2, b_pt):
        return True
    if abs(o3) <= eps and on_segment(a_pt, b_pt, p1):
        return True
    if abs(o4) <= eps and on_segment(a_pt, b_pt, p2):
        return True
    return (o1 > 0) != (o2 > 0) and (o3 > 0) != (o4 > 0)


def line_crossed(prev_point: Optional[Tuple[int, int]], curr_point: Tuple[int, int], line: List[Dict[str, int]]) -> bool:
    if not line or len(line) != 2:
        return True
    if prev_point is None:
        return False
    a, b = line
    if segments_intersect(prev_point, curr_point, a, b):
        return True
    # tolerate near-line entry when movement is small
    if point_dist_to_segment(curr_point, a, b) <= 3.0:
        return True
    return False


def direction_ok(direction: str, dx: float, dy: float) -> bool:
    direction = (direction or "any").strip().lower()
    if direction in ("", "any"):
        return True
    # Ignore tiny jitter
    if abs(dx) < 1e-3 and abs(dy) < 1e-3:
        return False
    mapping = {
        "left_to_right": (1.0, 0.0),
        "right_to_left": (-1.0, 0.0),
        "top_to_bottom": (0.0, 1.0),
        "bottom_to_top": (0.0, -1.0),
    }
    vec = mapping.get(direction)
    if not vec:
        return True
    return (dx * vec[0] + dy * vec[1]) > 0


def save_snapshot(frame: np.ndarray, stream_uuid: str, channel_id: str, track_id: str, object_class: str) -> str:
    timestamp = time.strftime("%Y%m%d-%H%M%S")
    target_dir = os.path.join(SNAPSHOT_DIR, stream_uuid, f"ch{channel_id}")
    os.makedirs(target_dir, exist_ok=True)
    filename = f"{timestamp}_{track_id}_{object_class}.jpg"
    path = os.path.join(target_dir, filename)
    cv2.imwrite(path, frame, [int(cv2.IMWRITE_JPEG_QUALITY), 85])
    return path


def format_event_time(iso_value: str) -> str:
    try:
        dt = datetime.fromisoformat(iso_value)
        return dt.strftime("%Y-%m-%d %H:%M:%S")
    except Exception:
        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def send_telegram(token: str, chat_id: str, caption: str, photo_path: Optional[str]) -> None:
    if not token or not chat_id:
        return
    try:
        if photo_path and os.path.isfile(photo_path):
            url = f"https://api.telegram.org/bot{token}/sendPhoto"
            with open(photo_path, "rb") as f:
                resp = requests.post(
                    url,
                    data={"chat_id": chat_id, "caption": caption},
                    files={"photo": f},
                    timeout=15,
                )
        else:
            url = f"https://api.telegram.org/bot{token}/sendMessage"
            resp = requests.post(
                url,
                data={"chat_id": chat_id, "text": caption},
                timeout=10,
            )
        if resp.status_code >= 300:
            app.logger.warning("telegram response: %s", resp.text)
    except Exception as exc:
        app.logger.error("telegram send error: %s", exc)


class YoloV5Detector:
    def __init__(self):
        if not download_model_if_needed():
            raise RuntimeError("model not available")
        self.net = cv2.dnn.readNetFromONNX(MODEL_PATH)

    def detect(
        self,
        frame: np.ndarray,
        allowed: set,
        conf_threshold: float,
        min_box_area: int,
    ) -> List[Tuple[str, Tuple[int, int], float]]:
        if frame is None:
            return []
        img, ratio, (dw, dh) = letterbox(frame, MODEL_INPUT)
        blob = cv2.dnn.blobFromImage(img, 1 / 255.0, (MODEL_INPUT, MODEL_INPUT), swapRB=True, crop=False)
        self.net.setInput(blob)
        outputs = self.net.forward()
        outputs = outputs[0] if len(outputs.shape) == 3 else outputs

        boxes = []
        confidences = []
        class_ids = []
        areas = []
        h, w = frame.shape[:2]
        for row in outputs:
            obj_conf = row[4]
            if obj_conf < conf_threshold:
                continue
            class_scores = row[5:]
            class_id = int(np.argmax(class_scores))
            conf = float(obj_conf * class_scores[class_id])
            if conf < conf_threshold:
                continue
            class_name = COCO_CLASSES[class_id] if class_id < len(COCO_CLASSES) else "unknown"
            if class_name not in allowed:
                continue
            x_center, y_center, width, height = row[0], row[1], row[2], row[3]
            x = (x_center - width / 2 - dw) / ratio
            y = (y_center - height / 2 - dh) / ratio
            box_w = width / ratio
            box_h = height / ratio
            x = max(0, min(w - 1, x))
            y = max(0, min(h - 1, y))
            box_w = max(1, min(w - x, box_w))
            box_h = max(1, min(h - y, box_h))
            area = box_w * box_h
            if min_box_area > 0 and area < min_box_area:
                continue
            boxes.append([int(x), int(y), int(box_w), int(box_h)])
            confidences.append(conf)
            class_ids.append(class_id)
            areas.append(area)

        if not boxes:
            return []
        indices = cv2.dnn.NMSBoxes(boxes, confidences, conf_threshold, IOU_THRESHOLD)
        detections = []
        if len(indices) == 0:
            return []
        for i in indices.flatten():
            x, y, bw, bh = boxes[i]
            centroid = (int(x + bw / 2), int(y + bh / 2))
            class_name = COCO_CLASSES[class_ids[i]]
            area = areas[i] if i < len(areas) else float(bw * bh)
            detections.append((class_name, centroid, area))
        return detections


class DetectorWorker(threading.Thread):
    def __init__(self, config: DetectionConfig, detector: YoloV5Detector, telegram_token: str):
        super().__init__(daemon=True)
        self.config = config
        self.detector = detector
        self.telegram_token = telegram_token
        self.stop_event = threading.Event()
        self.tracker = SimpleTracker()
        self.last_trigger_time = 0.0
        self.last_frame_at = None
        self.last_detection_count = 0
        self.last_error = ""
        self.frame_shape = None
        self._watchdog_last_frame_ts = 0.0
        self.frame_lock = threading.Lock()
        self.last_frame = None
        self.last_frame_ts = 0.0
        self.frame_buffer = deque(maxlen=6)
        self._last_detect_ts = 0.0
        self._consecutive_failures = 0
        self._reopen_backoff = RTSP_BACKOFF_BASE_SEC

    def update_config(self, config: DetectionConfig):
        self.config = config

    def stop(self):
        self.stop_event.set()

    def get_last_frame(self) -> Optional[np.ndarray]:
        with self.frame_lock:
            if self.last_frame is None:
                return None
            return self.last_frame.copy()

    def _pick_frame_for_event(self, target_ts: Optional[float]) -> Optional[np.ndarray]:
        if not self.frame_buffer:
            return None
        if target_ts is None:
            return self.frame_buffer[-1][1].copy()
        closest = min(self.frame_buffer, key=lambda item: abs(item[0] - target_ts))
        return closest[1].copy()

    def status(self) -> Dict[str, object]:
        return {
            "stream_uuid": self.config.stream_uuid,
            "channel_id": self.config.channel_id,
            "last_frame_at": self.last_frame_at,
            "last_detection_count": self.last_detection_count,
            "last_error": self.last_error,
            "frame_shape": self.frame_shape,
            "rtsp_transport": RTSP_TRANSPORT,
            "rtsp_read_timeout_sec": RTSP_READ_TIMEOUT_SEC,
            "rtsp_open_timeout_sec": RTSP_OPEN_TIMEOUT_SEC,
        }

    def run(self):
        cap = None
        while not self.stop_event.is_set():
            cfg = self.config
            if cfg.mode != "vehicle_entry":
                time.sleep(1)
                continue
            if self._watchdog_last_frame_ts and time.time() - self._watchdog_last_frame_ts > RTSP_READ_TIMEOUT_SEC:
                if cap is not None:
                    app.logger.warning("RTSP stalled; reopen")
                    cap.release()
                    cap = None
                self._watchdog_last_frame_ts = 0.0
            stream_url = cfg.stream_url
            if cap is None or not cap.isOpened():
                if RTSP_TRANSPORT:
                    options = [f"rtsp_transport;{RTSP_TRANSPORT}"]
                    if RTSP_OPEN_TIMEOUT_SEC > 0:
                        options.append(f"stimeout;{int(RTSP_OPEN_TIMEOUT_SEC * 1000000)}")
                    if RTSP_READ_TIMEOUT_SEC > 0:
                        options.append(f"rw_timeout;{int(RTSP_READ_TIMEOUT_SEC * 1000000)}")
                    os.environ["OPENCV_FFMPEG_CAPTURE_OPTIONS"] = "|".join(options)
                backend = cv2.CAP_FFMPEG if hasattr(cv2, "CAP_FFMPEG") else 0
                if backend:
                    cap = cv2.VideoCapture(stream_url, backend)
                else:
                    cap = cv2.VideoCapture(stream_url)
                cap.set(cv2.CAP_PROP_BUFFERSIZE, 1)
                if hasattr(cv2, "CAP_PROP_OPEN_TIMEOUT_MSEC"):
                    cap.set(cv2.CAP_PROP_OPEN_TIMEOUT_MSEC, int(RTSP_OPEN_TIMEOUT_SEC * 1000))
                if hasattr(cv2, "CAP_PROP_READ_TIMEOUT_MSEC"):
                    cap.set(cv2.CAP_PROP_READ_TIMEOUT_MSEC, int(RTSP_READ_TIMEOUT_SEC * 1000))
                if not cap.isOpened():
                    self.last_error = "RTSP open failed"
                    app.logger.warning("RTSP open failed: %s", stream_url)
                    self._consecutive_failures += 1
                    time.sleep(min(RTSP_BACKOFF_MAX_SEC, self._reopen_backoff))
                    self._reopen_backoff = min(RTSP_BACKOFF_MAX_SEC, max(RTSP_BACKOFF_BASE_SEC, self._reopen_backoff * 1.5))
                    continue
                self._consecutive_failures = 0
                self._reopen_backoff = RTSP_BACKOFF_BASE_SEC
                self.last_error = ""
            ok, frame = cap.read()
            if not ok or frame is None:
                self.last_error = "RTSP read failed"
                app.logger.warning("RTSP read failed; reopen")
                cap.release()
                cap = None
                self._consecutive_failures += 1
                if self._consecutive_failures >= RTSP_MAX_CONSEC_FAILS:
                    time.sleep(min(RTSP_BACKOFF_MAX_SEC, self._reopen_backoff))
                    self._reopen_backoff = min(RTSP_BACKOFF_MAX_SEC, max(RTSP_BACKOFF_BASE_SEC, self._reopen_backoff * 1.5))
                else:
                    time.sleep(RTSP_BACKOFF_BASE_SEC)
                continue
            self._consecutive_failures = 0
            self._reopen_backoff = RTSP_BACKOFF_BASE_SEC
            frame_ts = time.time()
            frame_copy = frame.copy()
            with self.frame_lock:
                self.last_frame = frame_copy
                self.last_frame_ts = frame_ts
            self.frame_buffer.append((frame_ts, frame_copy))
            self._watchdog_last_frame_ts = time.time()
            self.last_error = ""
            self.last_frame_at = datetime.now().astimezone().isoformat(timespec="seconds")
            self.frame_shape = list(frame.shape[:2])
            interval = max(0.2, 1.0 / max(1, cfg.sample_fps))
            if frame_ts - self._last_detect_ts < interval:
                continue
            self._last_detect_ts = frame_ts
            h, w = frame.shape[:2]
            polygon = normalize_polygon(cfg.polygon, w, h)
            entry_line = normalize_line(cfg.entry_line, w, h)
            if not polygon:
                continue
            conf_threshold = cfg.confidence_threshold if cfg.confidence_threshold > 0 else CONF_THRESHOLD
            allowed_classes = set(cfg.classes or [])
            if not allowed_classes:
                allowed_classes = DEFAULT_VEHICLE_CLASSES
            detections = self.detector.detect(frame, allowed_classes, conf_threshold, cfg.min_box_area)
            self.last_detection_count = len(detections)
            try:
                tracks = self.tracker.update(detections)
            except Exception as exc:
                self.last_error = f"tracker error: {exc}"
                app.logger.error("tracker error: %s", exc)
                time.sleep(1)
                continue
            now = time.time()
            for track in tracks:
                inside = point_in_polygon(track.centroid, polygon)
                if inside:
                    if not track.inside:
                        track.inside_streak = 1
                        track.first_inside_ts = time.time()
                    else:
                        track.inside_streak += 1
                else:
                    track.inside_streak = 0
                    track.seen_outside = True
                    track.triggered = False
                    track.last_outside_centroid = track.centroid
                    track.first_inside_ts = None
                track.inside = inside

                move_ok = True
                dx = 0.0
                dy = 0.0
                if cfg.min_move_px > 0:
                    dist = 0.0
                    if track.seen_outside and track.last_outside_centroid:
                        dx = track.centroid[0] - track.last_outside_centroid[0]
                        dy = track.centroid[1] - track.last_outside_centroid[1]
                        dist = math.hypot(dx, dy)
                    elif track.first_centroid:
                        dx = track.centroid[0] - track.first_centroid[0]
                        dy = track.centroid[1] - track.first_centroid[1]
                        dist = math.hypot(dx, dy)
                    move_ok = dist >= cfg.min_move_px
                elif track.prev_centroid:
                    dx = track.centroid[0] - track.prev_centroid[0]
                    dy = track.centroid[1] - track.prev_centroid[1]

                line_ok = True
                if entry_line:
                    prev_point = track.prev_centroid or track.last_outside_centroid
                    line_ok = line_crossed(prev_point, track.centroid, entry_line)
                allow_first_inside = (not entry_line) and (cfg.entry_direction or "any").strip().lower() in ("", "any")
                allow_first_inside = allow_first_inside or (entry_line and track.prev_centroid is not None)

                if (
                    inside
                    and line_ok
                    and track.inside_streak >= max(1, cfg.trigger_consecutive_frames)
                    and not track.triggered
                    and now - self.last_trigger_time >= max(1, cfg.cooldown_seconds)
                    and move_ok
                    and direction_ok(cfg.entry_direction, dx, dy)
                    and (track.seen_outside or allow_first_inside)
                ):
                    self.last_trigger_time = now
                    track.triggered = True
                    entered_at = datetime.now().astimezone().isoformat(timespec="seconds")
                    snapshot_frame = self._pick_frame_for_event(track.first_inside_ts)
                    if snapshot_frame is None:
                        snapshot_frame = frame
                    snapshot_path = save_snapshot(snapshot_frame, cfg.stream_uuid, cfg.channel_id, track.track_id, track.object_class)
                    self.send_event(track.object_class, track.track_id, entered_at, snapshot_path, cfg)
            # continue reading frames to keep snapshots near real-time

    def send_event(self, obj_class: str, track_id: str, entered_at: str, snapshot_path: str, cfg: DetectionConfig):
        payload = {
            "stream_uuid": cfg.stream_uuid,
            "channel_id": cfg.channel_id,
            "object_class": obj_class,
            "track_id": track_id,
            "entered_at": entered_at,
            "snapshot_path": snapshot_path,
        }
        try:
            resp = requests.post(
                f"{CAMLINK_API_BASE}/detector/events",
                headers={"X-CamLink-Detector-Token": CAMLINK_DETECTOR_TOKEN},
                json=payload,
                timeout=5,
            )
            if resp.status_code >= 300:
                app.logger.warning("event ingest failed: %s", resp.text)
        except Exception as exc:
            app.logger.error("event ingest error: %s", exc)

        if cfg.telegram_enabled and cfg.telegram_chat_id:
            human_time = format_event_time(entered_at)
            caption = f"#事件截图#\n[{cfg.stream_name}/CH{cfg.channel_id}]检测到外来车进入\n时间：{human_time}"
            send_telegram(self.telegram_token, cfg.telegram_chat_id, caption, snapshot_path)


class DetectorManager:
    def __init__(self):
        self.detector = YoloV5Detector()
        self.telegram_token = os.getenv("TELEGRAM_BOT_TOKEN", "")
        self.workers: Dict[str, DetectorWorker] = {}
        self.lock = threading.Lock()
        self.last_config = []
        self.last_error = ""

    def sync(self, configs: List[DetectionConfig]):
        with self.lock:
            active_keys = set()
            for cfg in configs:
                key = f"{cfg.stream_uuid}:{cfg.channel_id}"
                active_keys.add(key)
                if key in self.workers:
                    self.workers[key].update_config(cfg)
                else:
                    worker = DetectorWorker(cfg, self.detector, self.telegram_token)
                    self.workers[key] = worker
                    worker.start()
            # stop removed
            for key in list(self.workers.keys()):
                if key not in active_keys:
                    self.workers[key].stop()
                    del self.workers[key]
            self.last_config = configs

    def get_snapshot(self, stream_uuid: str, channel_id: str) -> Optional[np.ndarray]:
        key = f"{stream_uuid}:{channel_id}"
        with self.lock:
            worker = self.workers.get(key)
        if not worker:
            return None
        return worker.get_last_frame()

    def fetch_config(self) -> List[DetectionConfig]:
        if not CAMLINK_DETECTOR_TOKEN:
            self.last_error = "CAMLINK_DETECTOR_TOKEN is empty"
            return []
        try:
            resp = requests.get(
                f"{CAMLINK_API_BASE}/detector/config",
                headers={"X-CamLink-Detector-Token": CAMLINK_DETECTOR_TOKEN},
                timeout=5,
            )
            if resp.status_code >= 300:
                self.last_error = f"config fetch failed: {resp.status_code}"
                return []
            data = resp.json().get("payload", {})
            items = data.get("items", [])
            configs = []
            for item in items:
                det = item.get("detection", {})
                if not det:
                    continue
                configs.append(
                    DetectionConfig(
                        stream_uuid=item.get("stream_uuid", ""),
                        stream_name=item.get("stream_name", ""),
                        channel_id=item.get("channel_id", ""),
                        stream_url=item.get("stream_url", ""),
                        mode=det.get("mode", ""),
                        sample_fps=int(det.get("sample_fps", 1)),
                        cooldown_seconds=int(det.get("cooldown_seconds", 30)),
                        confidence_threshold=float(det.get("confidence_threshold", CONF_THRESHOLD)),
                        min_box_area=int(det.get("min_box_area", 0)),
                        min_move_px=int(det.get("min_move_px", 0)),
                        entry_direction=str(det.get("entry_direction", "any")),
                        entry_line=det.get("entry_line", []),
                        classes=det.get("classes", []),
                        polygon=det.get("polygon", []),
                        trigger_consecutive_frames=int(det.get("trigger_consecutive_frames", 2)),
                        telegram_enabled=bool(det.get("telegram_enabled", False)),
                        telegram_chat_id=det.get("telegram_chat_id", ""),
                    )
                )
            self.last_error = ""
            return configs
        except Exception as exc:
            self.last_error = str(exc)
            return []

    def run_loop(self):
        while True:
            configs = self.fetch_config()
            self.sync(configs)
            time.sleep(max(2.0, POLL_INTERVAL))


manager: Optional[DetectorManager] = None


def detector_token_ok() -> bool:
    token = request.headers.get("X-CamLink-Detector-Token", "").strip()
    if not token:
        token = request.args.get("token", "").strip()
    return token != "" and token == CAMLINK_DETECTOR_TOKEN


@app.get("/healthz")
def healthz():
    status = {
        "status": "ok",
        "message": "CamLink detector is running",
        "service": "camlink-detector",
        "time": time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime()),
    }
    if manager:
        status["active_streams"] = len(manager.workers)
        status["last_error"] = manager.last_error
        status["streams"] = [worker.status() for worker in manager.workers.values()]
    return jsonify(status)


@app.get("/v1/snapshot/<stream_uuid>/<channel_id>")
def snapshot(stream_uuid: str, channel_id: str):
    if not detector_token_ok():
        return jsonify({"status": "unauthorized"}), 401
    if not manager:
        return jsonify({"status": "not_ready"}), 503
    frame = manager.get_snapshot(stream_uuid, channel_id)
    if frame is None:
        return jsonify({"status": "not_found"}), 404
    ok, buf = cv2.imencode(".jpg", frame, [int(cv2.IMWRITE_JPEG_QUALITY), 85])
    if not ok:
        return jsonify({"status": "encode_failed"}), 500
    return Response(buf.tobytes(), mimetype="image/jpeg", headers={"Cache-Control": "no-store"})


@app.get("/v1/detector/info")
def info():
    return jsonify(
        {
            "service": "camlink-detector",
            "phase": "phase3-vehicle-entry",
            "capabilities": {
                "healthz": True,
                "vehicle_entry": True,
                "telegram_push": True,
                "event_storage": True,
            },
            "model": {
                "path": MODEL_PATH,
                "variant": MODEL_VARIANT,
                "url": MODEL_URL,
                "input": MODEL_INPUT,
                "conf": CONF_THRESHOLD,
                "iou": IOU_THRESHOLD,
            },
            "camlink": {
                "api_base": CAMLINK_API_BASE,
                "token_present": bool(CAMLINK_DETECTOR_TOKEN),
            },
        }
    )


if __name__ == "__main__":
    try:
        manager = DetectorManager()
        threading.Thread(target=manager.run_loop, daemon=True).start()
    except Exception as exc:
        app.logger.error("detector init failed: %s", exc)
    app.run(host="0.0.0.0", port=PORT)
