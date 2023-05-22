package worker

import (
	"context"
	"github.com/redis/go-redis/v9"
	authService "github.com/semmidev/auth-service"
	"github.com/semmidev/auth-service/mail"
	"github.com/semmidev/auth-service/token"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
)

type TaskProcessor interface {
	Start() error
	ProcessTaskSendOTPEmail(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server *asynq.Server

	store  authService.UserDataStore
	token  token.Maker
	mailer mail.EmailSender
}

func NewRedisTaskProcessor(redisOpt asynq.RedisClientOpt, store authService.UserDataStore, mailer mail.EmailSender) TaskProcessor {
	logger := NewLogger()
	redis.SetLogger(logger)

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			/*
			   QueueCritical adalah nama antrian yang mengindikasikan tugas-tugas yang memiliki tingkat kepentingan atau prioritas tinggi.
			   QueueDefault adalah nama antrian yang digunakan untuk tugas-tugas dengan tingkat kepentingan atau prioritas biasa.

			   10 dan 5) menentukan kapasitas maksimum atau jumlah tugas yang dapat diproses secara paralel dalam antrian tersebut.
			*/

			Queues: map[string]int{
				QueueCritical: 10,
				QueueDefault:  5,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().Err(err).Str("type", task.Type()).
					Bytes("payload", task.Payload()).Msg("process task failed")
			}),
			Logger: logger,
		},
	)

	return &RedisTaskProcessor{
		server: server,
		store:  store,
		mailer: mailer,
	}
}

func (processor *RedisTaskProcessor) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskSendOTPEmail, processor.ProcessTaskSendOTPEmail)
	return processor.server.Start(mux)
}
