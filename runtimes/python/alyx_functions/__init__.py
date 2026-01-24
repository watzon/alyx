"""
alyx_functions - Alyx Function SDK for Python

This module provides the SDK for writing serverless functions that run in Alyx.
"""

from __future__ import annotations

import json
import time
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Callable, TypeVar, Generic
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

T = TypeVar("T")


@dataclass
class AuthContext:
    """Contains authenticated user information."""

    id: str
    email: str
    role: str | None = None
    verified: bool = False
    metadata: dict[str, Any] | None = None


@dataclass
class LogEntry:
    """Represents a log entry from a function."""

    level: str
    message: str
    data: dict[str, Any] | None = None
    timestamp: str = field(default_factory=lambda: datetime.utcnow().isoformat() + "Z")


class Logger:
    """Structured logger for functions."""

    def __init__(self, logs: list[LogEntry]):
        self._logs = logs

    def _log(self, level: str, message: str, data: dict[str, Any] | None = None):
        self._logs.append(LogEntry(level=level, message=message, data=data or {}))

    def debug(self, message: str, data: dict[str, Any] | None = None):
        self._log("debug", message, data)

    def info(self, message: str, data: dict[str, Any] | None = None):
        self._log("info", message, data)

    def warn(self, message: str, data: dict[str, Any] | None = None):
        self._log("warn", message, data)

    def error(self, message: str, data: dict[str, Any] | None = None):
        self._log("error", message, data)


class CollectionClient:
    """Client for interacting with a specific collection."""

    def __init__(self, collection: str, alyx_url: str, internal_token: str):
        self._collection = collection
        self._alyx_url = alyx_url
        self._internal_token = internal_token

    def _request(
        self, path: str, method: str = "GET", data: dict | None = None
    ) -> dict:
        """Make an HTTP request to the Alyx internal API."""
        url = f"{self._alyx_url}{path}"
        headers = {
            "Authorization": f"Bearer {self._internal_token}",
            "Content-Type": "application/json",
        }

        body = json.dumps(data).encode("utf-8") if data else None
        req = Request(url, data=body, headers=headers, method=method)

        try:
            with urlopen(req, timeout=30) as response:
                return json.loads(response.read().decode("utf-8"))
        except HTTPError as e:
            error_body = e.read().decode("utf-8")
            try:
                error = json.loads(error_body)
                raise Exception(
                    error.get("message", f"Request failed with status {e.code}")
                )
            except json.JSONDecodeError:
                raise Exception(f"Request failed with status {e.code}")
        except URLError as e:
            raise Exception(f"Connection error: {e.reason}")

    def find(
        self,
        filter: dict[str, Any] | None = None,
        sort: str | None = None,
        limit: int | None = None,
        offset: int | None = None,
    ) -> list[dict]:
        """Find documents matching the filter."""
        params = [f"collection={self._collection}"]
        if filter:
            for key, value in filter.items():
                params.append(f"filter={key}:eq:{value}")
        if sort:
            params.append(f"sort={sort}")
        if limit:
            params.append(f"limit={limit}")
        if offset:
            params.append(f"offset={offset}")

        path = f"/internal/v1/db/query?{'&'.join(params)}"
        result = self._request(path)
        return result.get("data", [])

    def find_one(self, id: str) -> dict | None:
        """Find a single document by ID."""
        results = self.find(filter={"id": id})
        return results[0] if results else None

    def create(self, data: dict) -> dict:
        """Create a new document."""
        return self._request(
            "/internal/v1/db/exec",
            method="POST",
            data={"operation": "insert", "collection": self._collection, "data": data},
        )

    def update(self, id: str, data: dict) -> dict:
        """Update an existing document."""
        return self._request(
            "/internal/v1/db/exec",
            method="POST",
            data={
                "operation": "update",
                "collection": self._collection,
                "data": data,
                "id": id,
            },
        )

    def delete(self, id: str) -> dict:
        """Delete a document."""
        return self._request(
            "/internal/v1/db/exec",
            method="POST",
            data={"operation": "delete", "collection": self._collection, "id": id},
        )


class DbClient:
    """Database client that provides access to collections."""

    def __init__(self, alyx_url: str, internal_token: str):
        self._alyx_url = alyx_url
        self._internal_token = internal_token
        self._collections: dict[str, CollectionClient] = {}

    def __getattr__(self, collection: str) -> CollectionClient:
        if collection.startswith("_"):
            raise AttributeError(
                f"'{type(self).__name__}' object has no attribute '{collection}'"
            )
        if collection not in self._collections:
            self._collections[collection] = CollectionClient(
                collection, self._alyx_url, self._internal_token
            )
        return self._collections[collection]


@dataclass
class FunctionContext:
    """Context passed to function handlers."""

    auth: AuthContext | None
    env: dict[str, str]
    db: DbClient
    log: Logger


@dataclass
class FunctionError(Exception):
    """Error raised by functions."""

    code: str
    message: str
    details: dict[str, Any] | None = None


@dataclass
class FunctionDefinition(Generic[T]):
    """Definition of a serverless function."""

    handler: Callable[[dict, FunctionContext], T]
    input_schema: dict | None = None
    output_schema: dict | None = None


def define_function(
    handler: Callable[[dict, FunctionContext], T],
    input_schema: dict | None = None,
    output_schema: dict | None = None,
) -> FunctionDefinition[T]:
    """
    Define a serverless function.

    Args:
        handler: The function handler
        input_schema: Optional input validation schema
        output_schema: Optional output schema for codegen

    Returns:
        A FunctionDefinition
    """
    return FunctionDefinition(
        handler=handler,
        input_schema=input_schema,
        output_schema=output_schema,
    )


def validate_input(input_data: dict, schema: dict) -> None:
    """Validate input data against a schema."""
    for field_name, rules in schema.items():
        value = input_data.get(field_name)

        if rules.get("required") and value is None:
            raise FunctionError(
                code="VALIDATION_ERROR",
                message=f"Field '{field_name}' is required",
                details={"field": field_name},
            )

        if value is not None:
            expected_type = rules.get("type")
            if expected_type:
                type_map = {
                    "string": str,
                    "number": (int, float),
                    "boolean": bool,
                    "array": list,
                    "object": dict,
                }
                python_type = type_map.get(expected_type)
                if python_type and not isinstance(value, python_type):
                    raise FunctionError(
                        code="VALIDATION_ERROR",
                        message=f"Field '{field_name}' must be of type {expected_type}",
                        details={"field": field_name, "expected": expected_type},
                    )

            min_length = rules.get("min_length")
            if min_length and isinstance(value, str) and len(value) < min_length:
                raise FunctionError(
                    code="VALIDATION_ERROR",
                    message=f"Field '{field_name}' must be at least {min_length} characters",
                    details={"field": field_name, "min_length": min_length},
                )

            max_length = rules.get("max_length")
            if max_length and isinstance(value, str) and len(value) > max_length:
                raise FunctionError(
                    code="VALIDATION_ERROR",
                    message=f"Field '{field_name}' must be at most {max_length} characters",
                    details={"field": field_name, "max_length": max_length},
                )


def execute_function(function_def: FunctionDefinition, request: dict) -> dict:
    """
    Execute a function with the given request.

    Args:
        function_def: The function definition
        request: The function request

    Returns:
        The function response
    """
    logs: list[LogEntry] = []
    start_time = time.time()

    context_data = request.get("context", {})
    auth_data = context_data.get("auth")

    auth = None
    if auth_data:
        auth = AuthContext(
            id=auth_data.get("id", ""),
            email=auth_data.get("email", ""),
            role=auth_data.get("role"),
            verified=auth_data.get("verified", False),
            metadata=auth_data.get("metadata"),
        )

    db = DbClient(
        alyx_url=context_data.get("alyx_url", ""),
        internal_token=context_data.get("internal_token", ""),
    )

    context = FunctionContext(
        auth=auth,
        env=context_data.get("env", {}),
        db=db,
        log=Logger(logs),
    )

    try:
        # Validate input if schema is provided
        if function_def.input_schema:
            validate_input(request.get("input", {}), function_def.input_schema)

        # Execute the handler
        output = function_def.handler(request.get("input", {}), context)

        duration_ms = int((time.time() - start_time) * 1000)

        return {
            "request_id": request.get("request_id"),
            "success": True,
            "output": output,
            "logs": [
                {
                    "level": log.level,
                    "message": log.message,
                    "data": log.data,
                    "timestamp": log.timestamp,
                }
                for log in logs
            ],
            "duration_ms": duration_ms,
        }

    except FunctionError as e:
        duration_ms = int((time.time() - start_time) * 1000)
        return {
            "request_id": request.get("request_id"),
            "success": False,
            "error": {
                "code": e.code,
                "message": e.message,
                "details": e.details,
            },
            "logs": [
                {
                    "level": log.level,
                    "message": log.message,
                    "data": log.data,
                    "timestamp": log.timestamp,
                }
                for log in logs
            ],
            "duration_ms": duration_ms,
        }

    except Exception as e:
        duration_ms = int((time.time() - start_time) * 1000)
        return {
            "request_id": request.get("request_id"),
            "success": False,
            "error": {
                "code": "FUNCTION_ERROR",
                "message": str(e),
                "details": {},
            },
            "logs": [
                {
                    "level": log.level,
                    "message": log.message,
                    "data": log.data,
                    "timestamp": log.timestamp,
                }
                for log in logs
            ],
            "duration_ms": duration_ms,
        }


__all__ = [
    "define_function",
    "execute_function",
    "FunctionDefinition",
    "FunctionContext",
    "FunctionError",
    "AuthContext",
    "DbClient",
    "Logger",
]
