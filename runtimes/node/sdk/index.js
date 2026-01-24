/**
 * @alyx/functions - Alyx Function SDK for Node.js
 *
 * This module provides the SDK for writing serverless functions that run in Alyx.
 */

/**
 * Creates a database client for interacting with Alyx collections.
 * @param {object} context - The function context
 * @returns {object} Database client with collection methods
 */
function createDbClient(context) {
  const { alyxUrl, internalToken } = context;

  async function query(collection, options = {}) {
    const params = new URLSearchParams();
    if (options.filter) {
      for (const [key, value] of Object.entries(options.filter)) {
        params.append("filter", `${key}:eq:${value}`);
      }
    }
    if (options.sort) {
      params.append("sort", options.sort);
    }
    if (options.limit) {
      params.append("limit", String(options.limit));
    }
    if (options.offset) {
      params.append("offset", String(options.offset));
    }

    const url = `${alyxUrl}/internal/v1/db/query?collection=${collection}&${params}`;
    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${internalToken}`,
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || `Query failed with status ${response.status}`);
    }

    return response.json();
  }

  async function exec(operation, collection, data, id = null) {
    const url = `${alyxUrl}/internal/v1/db/exec`;
    const response = await fetch(url, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${internalToken}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ operation, collection, data, id }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || `Exec failed with status ${response.status}`);
    }

    return response.json();
  }

  // Create a proxy that returns collection clients
  return new Proxy(
    {},
    {
      get(_, collection) {
        return {
          async find(options = {}) {
            return query(collection, options);
          },

          async findOne(id) {
            const result = await query(collection, { filter: { id } });
            return result.data?.[0] || null;
          },

          async create(data) {
            return exec("insert", collection, data);
          },

          async update(id, data) {
            return exec("update", collection, data, id);
          },

          async delete(id) {
            return exec("delete", collection, null, id);
          },
        };
      },
    }
  );
}

/**
 * Creates a structured logger.
 * @param {Array} logs - Array to collect log entries
 * @returns {object} Logger with level methods
 */
function createLogger(logs) {
  const log = (level, message, data = {}) => {
    logs.push({
      level,
      message,
      data,
      timestamp: new Date().toISOString(),
    });
  };

  return {
    debug: (message, data) => log("debug", message, data),
    info: (message, data) => log("info", message, data),
    warn: (message, data) => log("warn", message, data),
    error: (message, data) => log("error", message, data),
  };
}

/**
 * Defines a serverless function with optional input/output schemas.
 * @param {object} config - Function configuration
 * @param {object} config.input - Input validation schema (optional)
 * @param {object} config.output - Output schema for codegen (optional)
 * @param {function} config.handler - The function handler
 * @returns {object} Function definition
 */
export function defineFunction(config) {
  return {
    input: config.input || null,
    output: config.output || null,
    handler: config.handler,
  };
}

/**
 * Executes a function with the given request.
 * @param {object} functionDef - The function definition from defineFunction
 * @param {object} request - The function request
 * @returns {Promise<object>} The function response
 */
export async function executeFunction(functionDef, request) {
  const logs = [];
  const startTime = Date.now();

  const context = {
    auth: request.context?.auth || null,
    env: request.context?.env || {},
    db: createDbClient({
      alyxUrl: request.context?.alyx_url,
      internalToken: request.context?.internal_token,
    }),
    log: createLogger(logs),
  };

  try {
    // Validate input if schema is provided
    if (functionDef.input) {
      validateInput(request.input, functionDef.input);
    }

    const output = await functionDef.handler(request.input || {}, context);

    return {
      request_id: request.request_id,
      success: true,
      output,
      logs,
      duration_ms: Date.now() - startTime,
    };
  } catch (error) {
    return {
      request_id: request.request_id,
      success: false,
      error: {
        code: error.code || "FUNCTION_ERROR",
        message: error.message,
        details: error.details || {},
      },
      logs,
      duration_ms: Date.now() - startTime,
    };
  }
}

/**
 * Validates input against a schema.
 * @param {object} input - The input to validate
 * @param {object} schema - The validation schema
 */
function validateInput(input, schema) {
  for (const [field, rules] of Object.entries(schema)) {
    const value = input?.[field];

    if (rules.required && (value === undefined || value === null)) {
      const error = new Error(`Field '${field}' is required`);
      error.code = "VALIDATION_ERROR";
      error.details = { field };
      throw error;
    }

    if (value !== undefined && value !== null) {
      if (rules.type && typeof value !== rules.type) {
        // Special handling for arrays
        if (rules.type === "array" && !Array.isArray(value)) {
          const error = new Error(`Field '${field}' must be an array`);
          error.code = "VALIDATION_ERROR";
          error.details = { field, expected: "array", got: typeof value };
          throw error;
        } else if (rules.type !== "array" && typeof value !== rules.type) {
          const error = new Error(`Field '${field}' must be of type ${rules.type}`);
          error.code = "VALIDATION_ERROR";
          error.details = { field, expected: rules.type, got: typeof value };
          throw error;
        }
      }

      if (rules.minLength && typeof value === "string" && value.length < rules.minLength) {
        const error = new Error(`Field '${field}' must be at least ${rules.minLength} characters`);
        error.code = "VALIDATION_ERROR";
        error.details = { field, minLength: rules.minLength };
        throw error;
      }

      if (rules.maxLength && typeof value === "string" && value.length > rules.maxLength) {
        const error = new Error(`Field '${field}' must be at most ${rules.maxLength} characters`);
        error.code = "VALIDATION_ERROR";
        error.details = { field, maxLength: rules.maxLength };
        throw error;
      }
    }
  }
}

export default { defineFunction, executeFunction };
