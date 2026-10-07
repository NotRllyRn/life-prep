package googlemail

import (
	"cloud.google.com/go/pubsub/v2"
	"context"
	"errors"
	"google.golang.org/api/option"
)

// Receive blocks until cancellation or a subscriber error. Handler failures
// nack the delivery for retry; handlers must tolerate duplicate deliveries.
// An empty credentialsFile uses Application Default Credentials.
func Receive(ctx context.Context, project, subscription, credentialsFile string, handler func(context.Context, []byte) error) error {
	if handler == nil {
		return errors.New("Pub/Sub handler is nil")
	}
	if project == "" || subscription == "" {
		return errors.New("Pub/Sub project and subscription are required")
	}
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	client, err := pubsub.NewClient(ctx, project, opts...)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Subscriber(subscription).Receive(ctx, func(ctx context.Context, m *pubsub.Message) { deliver(ctx, m.Data, handler, m.Ack, m.Nack) })
}
func deliver(ctx context.Context, data []byte, handler func(context.Context, []byte) error, ack, nack func()) {
	success := false
	defer func() {
		if success {
			ack()
		} else {
			nack()
		}
	}()
	success = handler(ctx, data) == nil
}
