#!/usr/bin/env python3
"""
Alyx Python Function Executor

This is the main executor that runs inside the container.
It listens for HTTP requests from the Alyx server and executes functions.
"""

from __future__ import annotations

import importlib.util
import json
import os
import signal
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from typing import Any

# Add the runtime directory to the path so we can import alyx_functions
sys.path.insert(0, "/runtime")

from alyx_functions import execute_function, FunctionDefinition

PORT = int(os.environ.get("PORT", "8080"))
FUNCTIONS_DIR = Path(os.environ.get("FUNCTIONS_DIR", "/functions"))

# Cache for loaded functions
function_cache: dict[str, FunctionDefinition] = {}


def find_function_entry(name: str) -> Path | None:
    """Find the entry file for a function (supports nested directories)."""
    func_dir = FUNCTIONS_DIR / name
    if func_dir.is_dir():
        index_path = func_dir / "index.py"
        if index_path.exists():
            return index_path

    direct_path = FUNCTIONS_DIR / f"{name}.py"
    if direct_path.exists():
        return direct_path

    return None


def load_function(name: str) -> FunctionDefinition:
    """Load a function module from the functions directory."""
    if name in function_cache:
        return function_cache[name]

    function_path = find_function_entry(name)
    if function_path is None:
        raise FileNotFoundError(f"Function '{name}' not found")

    spec = importlib.util.spec_from_file_location(name, function_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Could not load function '{name}'")

    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)

    if not hasattr(module, "default"):
        raise ValueError(
            f"Function '{name}' does not export a 'default' FunctionDefinition"
        )

    function_def = module.default
    if not isinstance(function_def, FunctionDefinition):
        raise ValueError(
            f"Function '{name}' default export is not a FunctionDefinition"
        )

    function_cache[name] = function_def
    return function_def


class RequestHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the executor."""

    def log_message(self, format: str, *args: Any) -> None:
        """Suppress default logging."""
        pass

    def send_json_response(self, status: int, data: dict) -> None:
        """Send a JSON response."""
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode("utf-8"))

    def do_GET(self) -> None:
        """Handle GET requests."""
        if self.path == "/health":
            self.send_json_response(200, {"status": "ok"})
        elif self.path == "/functions":
            self.handle_list_functions()
        else:
            self.send_json_response(404, {"error": "Not found"})

    def do_POST(self) -> None:
        """Handle POST requests."""
        if self.path == "/invoke":
            self.handle_invoke()
        elif self.path == "/clear-cache":
            self.handle_clear_cache()
        else:
            self.send_json_response(404, {"error": "Not found"})

    def handle_invoke(self) -> None:
        """Handle function invocation requests."""
        try:
            content_length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(content_length).decode("utf-8")
            request = json.loads(body)

            function_name = request.get("function")
            if not function_name:
                self.send_json_response(
                    400,
                    {
                        "success": False,
                        "error": {
                            "code": "INVALID_REQUEST",
                            "message": "Function name is required",
                        },
                    },
                )
                return

            # Load and execute the function
            function_def = load_function(function_name)
            response = execute_function(function_def, request)

            self.send_json_response(200, response)

        except FileNotFoundError as e:
            self.send_json_response(
                404,
                {
                    "success": False,
                    "error": {
                        "code": "FUNCTION_NOT_FOUND",
                        "message": str(e),
                    },
                },
            )
        except Exception as e:
            print(f"Invoke error: {e}", file=sys.stderr)
            self.send_json_response(
                500,
                {
                    "success": False,
                    "error": {
                        "code": "EXECUTOR_ERROR",
                        "message": str(e),
                    },
                },
            )

    def handle_clear_cache(self) -> None:
        """Clear the function cache."""
        function_cache.clear()
        self.send_json_response(200, {"status": "cache_cleared"})

    def handle_list_functions(self) -> None:
        """List available functions."""
        try:
            if not FUNCTIONS_DIR.exists():
                self.send_json_response(200, {"functions": []})
                return

            functions = []
            for entry in FUNCTIONS_DIR.iterdir():
                if entry.name.startswith("_") or entry.name.startswith("."):
                    continue
                if entry.is_dir():
                    if find_function_entry(entry.name) is not None:
                        functions.append(entry.name)
                elif entry.suffix == ".py":
                    functions.append(entry.stem)

            self.send_json_response(200, {"functions": functions})
        except Exception as e:
            self.send_json_response(
                500,
                {
                    "error": {
                        "code": "LIST_ERROR",
                        "message": str(e),
                    }
                },
            )


def run_server() -> None:
    """Run the HTTP server."""
    server = HTTPServer(("0.0.0.0", PORT), RequestHandler)
    print(f"Alyx Python executor listening on port {PORT}")

    def shutdown(signum: int, frame: Any) -> None:
        print(f"Received signal {signum}, shutting down...")
        server.shutdown()
        sys.exit(0)

    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    server.serve_forever()


if __name__ == "__main__":
    run_server()
