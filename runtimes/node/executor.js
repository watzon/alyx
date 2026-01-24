/**
 * Alyx Node.js Function Executor
 *
 * This is the main executor that runs inside the container.
 * It listens for HTTP requests from the Alyx server and executes functions.
 */

import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { executeFunction } from "./sdk/index.js";

const PORT = process.env.PORT || 8080;
const FUNCTIONS_DIR = process.env.FUNCTIONS_DIR || "/functions";

// Cache for loaded functions
const functionCache = new Map();

/**
 * Load a function module from the functions directory.
 * @param {string} name - Function name
 * @returns {Promise<object>} The function definition
 */
async function loadFunction(name) {
  // Check cache first
  if (functionCache.has(name)) {
    return functionCache.get(name);
  }

  // Try different file extensions
  const extensions = [".js", ".mjs", ".cjs"];
  let functionPath = null;

  for (const ext of extensions) {
    const testPath = path.join(FUNCTIONS_DIR, `${name}${ext}`);
    if (fs.existsSync(testPath)) {
      functionPath = testPath;
      break;
    }
  }

  if (!functionPath) {
    throw new Error(`Function '${name}' not found`);
  }

  // Import the function module
  const moduleUrl = pathToFileURL(functionPath).href;
  const module = await import(moduleUrl);

  // Get the function definition (default export)
  const functionDef = module.default;
  if (!functionDef || typeof functionDef.handler !== "function") {
    throw new Error(`Function '${name}' does not export a valid function definition`);
  }

  // Cache the function
  functionCache.set(name, functionDef);

  return functionDef;
}

/**
 * Handle health check requests.
 * @param {http.ServerResponse} res
 */
function handleHealth(res) {
  res.writeHead(200, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ status: "ok" }));
}

/**
 * Handle function invocation requests.
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 */
async function handleInvoke(req, res) {
  let body = "";

  req.on("data", (chunk) => {
    body += chunk;
  });

  req.on("end", async () => {
    try {
      const request = JSON.parse(body);
      const { function: functionName } = request;

      if (!functionName) {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(
          JSON.stringify({
            success: false,
            error: {
              code: "INVALID_REQUEST",
              message: "Function name is required",
            },
          })
        );
        return;
      }

      // Load and execute the function
      const functionDef = await loadFunction(functionName);
      const response = await executeFunction(functionDef, request);

      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify(response));
    } catch (error) {
      console.error("Invoke error:", error);

      res.writeHead(500, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          success: false,
          error: {
            code: "EXECUTOR_ERROR",
            message: error.message,
          },
        })
      );
    }
  });
}

/**
 * Clear the function cache (for hot reloading).
 */
function handleClearCache(res) {
  functionCache.clear();
  res.writeHead(200, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ status: "cache_cleared" }));
}

/**
 * List available functions.
 */
function handleListFunctions(res) {
  try {
    if (!fs.existsSync(FUNCTIONS_DIR)) {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ functions: [] }));
      return;
    }

    const files = fs.readdirSync(FUNCTIONS_DIR);
    const functions = files
      .filter((f) => f.endsWith(".js") || f.endsWith(".mjs") || f.endsWith(".cjs"))
      .filter((f) => !f.startsWith("_")) // Exclude shared modules
      .map((f) => path.basename(f, path.extname(f)));

    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ functions }));
  } catch (error) {
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        error: {
          code: "LIST_ERROR",
          message: error.message,
        },
      })
    );
  }
}

// Create the HTTP server
const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);

  if (req.method === "GET" && url.pathname === "/health") {
    handleHealth(res);
  } else if (req.method === "POST" && url.pathname === "/invoke") {
    handleInvoke(req, res);
  } else if (req.method === "POST" && url.pathname === "/clear-cache") {
    handleClearCache(res);
  } else if (req.method === "GET" && url.pathname === "/functions") {
    handleListFunctions(res);
  } else {
    res.writeHead(404, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Not found" }));
  }
});

// Handle graceful shutdown
process.on("SIGTERM", () => {
  console.log("Received SIGTERM, shutting down...");
  server.close(() => {
    process.exit(0);
  });
});

process.on("SIGINT", () => {
  console.log("Received SIGINT, shutting down...");
  server.close(() => {
    process.exit(0);
  });
});

// Start the server
server.listen(PORT, "0.0.0.0", () => {
  console.log(`Alyx Node.js executor listening on port ${PORT}`);
});
