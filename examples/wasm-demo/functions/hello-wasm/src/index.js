function handle() {
  const inputJson = Host.inputString();
  const req = JSON.parse(inputJson);
  
  const now = new Date();
  
  const input = req.input || {};
  const userName = input.name || 'World';
  
  const response = {
    success: true,
    message: `Hello, ${userName}! 🎉`,
    timestamp: now.toISOString(),
    metadata: {
      function: req.function_name,
      request_id: req.request_id,
      runtime: 'wasm'
    }
  };
  
  Host.outputString(JSON.stringify(response));
  return 0;
}

module.exports = { handle };
