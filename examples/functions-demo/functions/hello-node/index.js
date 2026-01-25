#!/usr/bin/env node

/**
 * Hello Node Function
 * 
 * Demonstrates the JSON stdin/stdout protocol for Alyx functions.
 * Reads FunctionRequest from stdin, processes it, writes FunctionResponse to stdout.
 */

const readline = require('readline');

/**
 * Main entry point - reads JSON from stdin and writes JSON to stdout
 */
async function main() {
  try {
    // Read entire stdin as JSON
    const input = await readStdin();
    const request = JSON.parse(input);

    // Validate request structure
    if (!request.request_id || !request.function || !request.input) {
      throw new Error('Invalid request: missing required fields (request_id, function, input)');
    }

    // Process the request
    const name = request.input.name || 'World';
    const message = `Hello, ${name}! (from Node.js)`;

    // Build response
    const response = {
      request_id: request.request_id,
      success: true,
      output: {
        message: message,
        timestamp: new Date().toISOString(),
        runtime: 'node',
        version: process.version
      }
    };

    // Write response to stdout
    console.log(JSON.stringify(response));
    process.exit(0);

  } catch (error) {
    // Error response
    const errorResponse = {
      request_id: 'unknown',
      success: false,
      error: {
        message: error.message,
        stack: error.stack
      }
    };

    console.log(JSON.stringify(errorResponse));
    process.exit(1);
  }
}

/**
 * Read all data from stdin
 * @returns {Promise<string>} Complete stdin content
 */
function readStdin() {
  return new Promise((resolve, reject) => {
    let data = '';

    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
      terminal: false
    });

    rl.on('line', (line) => {
      data += line;
    });

    rl.on('close', () => {
      resolve(data);
    });

    rl.on('error', (err) => {
      reject(err);
    });
  });
}

// Run main function
main();
