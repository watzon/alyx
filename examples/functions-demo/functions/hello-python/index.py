#!/usr/bin/env python3

"""
Hello Python Function

Demonstrates the JSON stdin/stdout protocol for Alyx functions.
Reads FunctionRequest from stdin, processes it, writes FunctionResponse to stdout.
"""

import sys
import json
from datetime import datetime


def main():
    """Main entry point - reads JSON from stdin and writes JSON to stdout"""
    try:
        # Read entire stdin as JSON
        input_data = sys.stdin.read()
        request = json.loads(input_data)

        # Validate request structure
        if not all(k in request for k in ["request_id", "function", "input"]):
            raise ValueError(
                "Invalid request: missing required fields (request_id, function, input)"
            )

        # Process the request
        name = request["input"].get("name", "World")
        message = f"Hello, {name}! (from Python)"

        # Build response
        response = {
            "request_id": request["request_id"],
            "success": True,
            "output": {
                "message": message,
                "timestamp": datetime.utcnow().isoformat() + "Z",
                "runtime": "python",
                "version": sys.version.split()[0],
            },
        }

        # Write response to stdout
        print(json.dumps(response))
        sys.exit(0)

    except Exception as error:
        # Error response
        error_response = {
            "request_id": "unknown",
            "success": False,
            "error": str(error),
            "error_code": "EXECUTION_ERROR",
        }

        print(json.dumps(error_response))
        sys.exit(1)


if __name__ == "__main__":
    main()
