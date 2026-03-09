import os
from datetime import datetime, timezone
from flask import Flask, jsonify

app = Flask(__name__)

PORT = int(os.getenv("CAMLINK_DETECTOR_PORT", "8091"))


@app.get("/healthz")
def healthz():
    return jsonify(
        {
            "status": "ok",
            "message": "CamLink detector skeleton is running",
            "service": "camlink-detector",
            "time": datetime.now(timezone.utc).isoformat(),
        }
    )


@app.get("/v1/detector/info")
def info():
    return jsonify(
        {
            "service": "camlink-detector",
            "phase": "phase1-skeleton",
            "capabilities": {
                "healthz": True,
                "vehicle_entry": False,
                "telegram_push": False,
                "event_storage": False,
            },
        }
    )


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=PORT)
