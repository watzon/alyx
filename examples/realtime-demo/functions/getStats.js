/**
 * getStats - Returns message statistics across channels.
 *
 * Input: { channel?: string }
 * Output: { totalMessages: number, channels: object, recentAuthors: string[] }
 */

export default {
  input: {
    channel: { type: "string" },
  },

  async handler(input, context) {
    const { channel } = input;

    const filter = channel ? { channel } : {};
    const messages = await context.db.messages.find({ filter, limit: 1000 });

    const channelCounts = {};
    const authorSet = new Set();

    for (const msg of messages.data || []) {
      channelCounts[msg.channel] = (channelCounts[msg.channel] || 0) + 1;
      authorSet.add(msg.author);
    }

    const recentMessages = await context.db.messages.find({
      filter,
      sort: "-created_at",
      limit: 10,
    });

    const recentAuthors = [
      ...new Set((recentMessages.data || []).map((m) => m.author)),
    ];

    context.log.info("Stats computed", {
      total: messages.data?.length || 0,
      channelCount: Object.keys(channelCounts).length,
    });

    return {
      totalMessages: messages.data?.length || 0,
      channels: channelCounts,
      recentAuthors,
    };
  },
};
