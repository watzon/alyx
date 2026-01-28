#!/usr/bin/env python3

import sys
import json
import base64
import hashlib


def get_text_preview(
    data: bytes, content_type: str, max_length: int = 500
) -> str | None:
    text_types = [
        "text/",
        "application/json",
        "application/xml",
        "application/javascript",
    ]
    if not any(content_type.startswith(t) for t in text_types):
        return None

    try:
        text = data.decode("utf-8")
        return text[:max_length] + "..." if len(text) > max_length else text
    except UnicodeDecodeError:
        return None


def analyze_file(file_info: dict) -> dict:
    data = base64.b64decode(file_info["data"])

    result = {
        "filename": file_info["filename"],
        "content_type": file_info["content_type"],
        "size": file_info["size"],
        "md5": hashlib.md5(data).hexdigest(),
        "sha256": hashlib.sha256(data).hexdigest(),
    }

    preview = get_text_preview(data, file_info["content_type"])
    if preview:
        result["preview"] = preview

    return result


def main():
    request = None
    try:
        input_data = sys.stdin.read()
        request = json.loads(input_data)

        if not all(k in request for k in ["request_id", "function", "input"]):
            raise ValueError("Invalid request: missing required fields")

        files = request["input"].get("_files", [])

        if not files:
            response = {
                "request_id": request["request_id"],
                "success": True,
                "output": {
                    "message": "No files uploaded. Use multipart/form-data to upload files.",
                    "files": [],
                },
            }
        else:
            analyzed = [analyze_file(f) for f in files]
            total_size = sum(f["size"] for f in files)

            response = {
                "request_id": request["request_id"],
                "success": True,
                "output": {
                    "message": f"Analyzed {len(files)} file(s)",
                    "total_size": total_size,
                    "files": analyzed,
                },
            }

        print(json.dumps(response))
        sys.exit(0)

    except Exception as error:
        error_response = {
            "request_id": request.get("request_id", "unknown")
            if request
            else "unknown",
            "success": False,
            "error": {"code": "EXECUTION_ERROR", "message": str(error)},
        }
        print(json.dumps(error_response))
        sys.exit(1)


if __name__ == "__main__":
    main()
