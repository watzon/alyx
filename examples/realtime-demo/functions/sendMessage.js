/**
 * sendMessage - Creates a new chat message with optional author auto-generation.
 *
 * Input: { channel: string, content: string, author?: string }
 * Output: { id: string, channel: string, author: string, content: string }
 */

export default {
  input: {
    channel: { type: "string", required: true, minLength: 1, maxLength: 50 },
    content: { type: "string", required: true, minLength: 1, maxLength: 2000 },
    author: { type: "string", maxLength: 100 },
  },

  async handler(input, context) {
    const { channel, content, author } = input;

    const finalAuthor =
      author || `Anon-${Math.random().toString(36).slice(2, 6)}`;

    const result = await context.db.messages.create({
      channel,
      content,
      author: finalAuthor,
    });

    context.log.info("Message created", {
      id: result.id,
      channel,
      author: finalAuthor,
    });

    return {
      id: result.id,
      channel,
      author: finalAuthor,
      content,
    };
  },
};
