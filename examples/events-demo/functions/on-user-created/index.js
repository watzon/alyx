import { getContext } from '../../sdk';

export default async function handler(req, res) {
  const { alyx, env } = getContext();
  const { document, collection, action } = req.input;

  console.log(`New user created: ${document.email}`);

  try {
    await alyx.collections.notifications.create({
      user_id: document.id,
      type: 'welcome',
      title: 'Welcome to Alyx!',
      message: `Hi ${document.name}, thanks for signing up!`,
      read: false,
    });

    console.log(`Welcome notification created for ${document.email}`);

    return res.json({
      success: true,
      user_id: document.id,
      notification_sent: true,
    });
  } catch (error) {
    console.error('Failed to create notification:', error);
    return res.json({
      success: false,
      error: error.message,
    }, 500);
  }
}
