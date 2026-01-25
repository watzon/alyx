#!/usr/bin/env node

/**
 * Hello TypeScript Function
 * 
 * Demonstrates the JSON stdin/stdout protocol with TypeScript types.
 * Reads FunctionRequest from stdin, processes it, writes FunctionResponse to stdout.
 */

interface FunctionRequest {
  request_id: string;
  function: string;
  input: Record<string, any>;
  context: {
    auth?: any;
    env?: Record<string, string>;
    alyx_url: string;
    internal_token: string;
  };
}

interface FunctionResponse {
  request_id: string;
  success: boolean;
  output?: any;
  error?: {
    code: string;
    message: string;
  };
}

/**
 * Main entry point - reads JSON from stdin and writes JSON to stdout
 */
async function main() {
  try {
    // Read entire stdin as JSON
    const chunks: Buffer[] = [];
    
    for await (const chunk of process.stdin) {
      chunks.push(chunk);
    }
    
    const input = Buffer.concat(chunks).toString('utf-8');
    const request: FunctionRequest = JSON.parse(input);

    // Validate request structure
    if (!request.request_id || !request.function || !request.input) {
      throw new Error('Invalid request: missing required fields (request_id, function, input)');
    }

    // Process the request
    const name = request.input.name || 'World';
    const message = `Hello, ${name}! (from TypeScript)`;

    // Build response
    const response: FunctionResponse = {
      request_id: request.request_id,
      success: true,
      output: {
        message: message,
        timestamp: new Date().toISOString(),
        runtime: 'node',
        language: 'typescript',
        version: process.version,
        mode: process.env.NODE_ENV || 'development'
      }
    };

    // Write response to stdout
    console.log(JSON.stringify(response));
    process.exit(0);

  } catch (error) {
    // Error response
    const errorResponse: FunctionResponse = {
      request_id: 'unknown',
      success: false,
      error: {
        code: 'INTERNAL_ERROR',
        message: error instanceof Error ? error.message : String(error)
      }
    };

    console.log(JSON.stringify(errorResponse));
    process.exit(1);
  }
}

// Run main function
main();
