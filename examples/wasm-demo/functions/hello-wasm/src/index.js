import { format, formatDistance, formatRelative } from 'date-fns';

/**
 * Main function handler for the hello-wasm example.
 * Demonstrates using npm packages (date-fns) in WASM functions.
 * 
 * @param {FunctionRequest} req - The function request object
 * @returns {FunctionResponse} The function response
 */
export function handle(req) {
  const now = new Date();
  const pastDate = new Date(2026, 0, 1); // January 1, 2026
  
  // Use date-fns to format dates in various ways
  const formatted = format(now, 'yyyy-MM-dd HH:mm:ss');
  const distance = formatDistance(pastDate, now, { addSuffix: true });
  const relative = formatRelative(pastDate, now);
  
  // Access request data
  const input = req.input || {};
  const userName = input.name || 'World';
  
  // Access function context
  const functionName = req.function_name;
  const requestId = req.request_id;
  
  // Build response
  return {
    success: true,
    message: `Hello, ${userName}! 🎉`,
    timestamp: {
      formatted: formatted,
      distance: distance,
      relative: relative,
      iso: now.toISOString()
    },
    metadata: {
      function: functionName,
      request_id: requestId,
      runtime: 'wasm',
      npm_package: 'date-fns@3.0.0'
    },
    example_usage: {
      description: 'You can pass data to this function via the input field',
      example: {
        name: 'Alice'
      }
    }
  };
}
