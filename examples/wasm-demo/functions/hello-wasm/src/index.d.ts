/**
 * Type definitions for Alyx WASM function runtime.
 * These types describe the request/response structure for WASM functions.
 */

/**
 * Authentication context provided by Alyx.
 */
interface AuthContext {
  /** User ID if authenticated, null otherwise */
  id?: string;
  /** User email if authenticated */
  email?: string;
  /** User role if authenticated */
  role?: string;
  /** Custom user metadata */
  metadata?: Record<string, any>;
}

/**
 * Function execution context provided by Alyx.
 */
interface FunctionContext {
  /** Authentication information */
  auth?: AuthContext;
  /** Environment variables */
  env: Record<string, string>;
  /** Internal Alyx API URL */
  alyx_url: string;
  /** Internal API token for calling Alyx APIs */
  internal_token: string;
}

/**
 * Function request structure.
 */
interface FunctionRequest {
  /** Unique request ID */
  request_id: string;
  /** Function name being invoked */
  function_name: string;
  /** Input data passed to the function */
  input?: any;
  /** Execution context */
  context: FunctionContext;
}

/**
 * Function response structure.
 */
interface FunctionResponse {
  /** Response data (will be JSON serialized) */
  [key: string]: any;
}

/**
 * Main function handler.
 * This is the entry point for your WASM function.
 * 
 * @param req - The function request object
 * @returns The function response object
 */
export function handle(req: FunctionRequest): FunctionResponse;
