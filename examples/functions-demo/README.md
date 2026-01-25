# Functions Demo

This example demonstrates Alyx's polyglot serverless functions using the subprocess runtime.

## Included Functions

- **hello-deno** - Deno TypeScript function
- **hello-node** - Node.js JavaScript function  
- **hello-python** - Python 3 function

## Running

```bash
# From the alyx project root
make build

# Run the dev server
cd examples/functions-demo
../../build/alyx dev
```

## Testing Functions

```bash
# Test Deno function
curl -X POST http://localhost:8090/api/functions/hello-deno \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'

# Test Node function
curl -X POST http://localhost:8090/api/functions/hello-node \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'

# Test Python function
curl -X POST http://localhost:8090/api/functions/hello-python \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'
```

## Function Structure

Each function:
1. Reads `FunctionRequest` JSON from stdin
2. Processes the input
3. Writes `FunctionResponse` JSON to stdout

See individual function directories for implementation details.
