package devices

import "context"

type key struct{}

var deviceKey key

func NewContext(ctx context.Context, device *Device) context.Context {
	return context.WithValue(ctx, deviceKey, device)
}

func FromContext(ctx context.Context) (*Device, bool) {
	d, ok := ctx.Value(deviceKey).(*Device)
	return d, ok
}
