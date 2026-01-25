#!/usr/bin/env -S deno run --allow-all

/**
 * Hello Deno Function
 * 
 * Demonstrates the JSON stdin/stdout protocol for Alyx functions.
 * Reads FunctionRequest from stdin, processes it, writes FunctionResponse to stdout.
 */

interface FunctionRequest {
  request_id: string;
  function: string;
  input: Record<string, any>;
  context?: {
    auth?: any;
    env?: Record<string, string>;
    alyx_url?: string;
    internal_token?: string;
  };
}

interface FunctionResponse {
  request_id: string;
  success: boolean;
  output?: any;
  error?: string;
  error_code?: string;
}

/**
 * Main entry point - reads JSON from stdin and writes JSON to stdout
 */
async function main() {
  try {
    // Read entire stdin as text
    const chunks: Uint8Array[] = [];
    for await (const chunk of Deno.stdin.readable) {
      chunks.push(chunk);
    }
    const decoder = new TextDecoder();
    const input = decoder.decode(new Uint8Array(chunks.flatMap(c => Array.from(c))));
    
    // Parse JSON request
    const request: FunctionRequest = JSON.parse(input);

    // Validate request structure
    if (!request.request_id || !request.function || !request.input) {
      throw new Error('Invalid request: missing required fields (request_id, function, input)');
    }

    // Process the request
    const name = request.input.name || 'World';
    const message = `Hello, ${name}! (from Deno)`;

    // Build response
    const response: FunctionResponse = {
      request_id: request.request_id,
      success: true,
      output: {
        message: message,
        timestamp: new Date().toISOString(),
        runtime: 'deno',
        version: Deno.version.deno
      }
    };

    // Write response to stdout
    console.log(JSON.stringify(response));
    Deno.exit(0);

  } catch (error) {
    // Error response
    const errorResponse: FunctionResponse = {
      request_id: 'unknown',
      success: false,
      error: error instanceof Error ? error.message : String(error),
      error_code: 'EXECUTION_ERROR'
    };

    console.log(JSON.stringify(errorResponse));
    Deno.exit(1);
  }
}

// Run main function
main();
