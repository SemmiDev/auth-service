package worker

import (
	"context"
	"github.com/semmidev/auth-service/token"

	"github.com/hibiken/asynq"
)

type TaskDistributor interface {
	DistributeTaskSendOTPEmail(
		ctx context.Context,
		payload *PayloadSendOTPEmail,
		opts ...asynq.Option,
	) error
}

type RedisTaskDistributor struct {
	client *asynq.Client
	token  token.Maker
}

func NewRedisTaskDistributor(redisOpt asynq.RedisClientOpt, token token.Maker) TaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{
		client: client,
		token:  token,
	}
}
