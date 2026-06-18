package repository

import "context"

type IpRepository interface {
	Add(ctx context.Context, ip string, reason string) error
	Exists(ctx context.Context, ip string) (bool, error)
}
