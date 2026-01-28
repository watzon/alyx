interface FunctionRequest {
  request_id: string;
  function: string;
  input: { name?: string };
  context: any;
}

interface FunctionResponse {
  request_id: string;
  success: boolean;
  output?: any;
}

const input = await Bun.stdin.text();
const request: FunctionRequest = JSON.parse(input);

const name = request.input.name || "World";
const message = `Hello, ${name}! (from TypeScript)`;

const response: FunctionResponse = {
  request_id: request.request_id,
  success: true,
  output: { message, timestamp: new Date().toISOString() },
};

console.log(JSON.stringify(response));
