import { getContext } from '../../sdk';

export default async function handler(req, res) {
  const { alyx } = getContext();
  const { body, verified, verification_error } = req.input;

  if (!verified) {
    console.error('Webhook verification failed:', verification_error);
    return res.json({
      error: 'Invalid signature',
      details: verification_error,
    }, 401);
  }

  const event = JSON.parse(body);
  console.log(`Received Stripe event: ${event.type}`);

  try {
    switch (event.type) {
      case 'charge.succeeded': {
        const charge = event.data.object;
        
        await alyx.collections.payments.create({
          stripe_charge_id: charge.id,
          amount: charge.amount,
          currency: charge.currency,
          customer_email: charge.billing_details?.email,
          status: 'succeeded',
          created_at: new Date(charge.created * 1000).toISOString(),
        });

        console.log(`Payment recorded: ${charge.id}`);
        break;
      }

      case 'charge.failed': {
        const charge = event.data.object;
        
        await alyx.collections.payments.create({
          stripe_charge_id: charge.id,
          amount: charge.amount,
          currency: charge.currency,
          customer_email: charge.billing_details?.email,
          status: 'failed',
          failure_message: charge.failure_message,
          created_at: new Date(charge.created * 1000).toISOString(),
        });

        console.log(`Failed payment recorded: ${charge.id}`);
        break;
      }

      case 'customer.subscription.created':
      case 'customer.subscription.updated':
      case 'customer.subscription.deleted': {
        const subscription = event.data.object;
        
        await alyx.collections.subscriptions.upsert({
          stripe_subscription_id: subscription.id,
          customer_id: subscription.customer,
          status: subscription.status,
          current_period_start: new Date(subscription.current_period_start * 1000).toISOString(),
          current_period_end: new Date(subscription.current_period_end * 1000).toISOString(),
          cancel_at_period_end: subscription.cancel_at_period_end,
        });

        console.log(`Subscription ${event.type}: ${subscription.id}`);
        break;
      }

      default:
        console.log(`Unhandled event type: ${event.type}`);
    }

    return res.json({ received: true });
  } catch (error) {
    console.error('Error processing webhook:', error);
    return res.json({
      error: 'Processing failed',
      details: error.message,
    }, 500);
  }
}
