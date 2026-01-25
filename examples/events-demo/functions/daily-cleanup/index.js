import { getContext } from '../../sdk';

export default async function handler(req, res) {
  const { alyx } = getContext();
  const { retention_days = 30 } = req.input;

  console.log(`Starting cleanup with ${retention_days} day retention`);

  const cutoffDate = new Date();
  cutoffDate.setDate(cutoffDate.getDate() - retention_days);
  const cutoff = cutoffDate.toISOString();

  try {
    const results = {
      executions: 0,
      notifications: 0,
      sessions: 0,
    };

    const executionsDeleted = await alyx.collections.executions.deleteMany({
      created_at: { $lt: cutoff },
      status: { $in: ['success', 'failed'] },
    });
    results.executions = executionsDeleted.count;
    console.log(`Deleted ${results.executions} old execution logs`);

    const notificationsDeleted = await alyx.collections.notifications.deleteMany({
      created_at: { $lt: cutoff },
      read: true,
    });
    results.notifications = notificationsDeleted.count;
    console.log(`Deleted ${results.notifications} read notifications`);

    const sessionsDeleted = await alyx.collections.sessions.deleteMany({
      expires_at: { $lt: new Date().toISOString() },
    });
    results.sessions = sessionsDeleted.count;
    console.log(`Deleted ${results.sessions} expired sessions`);

    console.log('Cleanup completed successfully');

    return res.json({
      success: true,
      retention_days,
      cutoff_date: cutoff,
      deleted: results,
      total_deleted: results.executions + results.notifications + results.sessions,
    });
  } catch (error) {
    console.error('Cleanup failed:', error);
    return res.json({
      success: false,
      error: error.message,
    }, 500);
  }
}
