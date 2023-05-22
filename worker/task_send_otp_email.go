package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	authService "github.com/semmidev/auth-service"
	"github.com/semmidev/auth-service/token"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const TaskSendOTPEmail = "task:send_otp_email"

type PayloadSendOTPEmail struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func (distributor *RedisTaskDistributor) DistributeTaskSendOTPEmail(
	ctx context.Context,
	payload *PayloadSendOTPEmail,
	opts ...asynq.Option,
) error {
	generatedToken, _, err := distributor.token.CreateToken(payload.Email, token.OTP, 5*time.Minute)
	if err != nil {
		return err
	}

	payload.Token = generatedToken

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	task := asynq.NewTask(TaskSendOTPEmail, jsonPayload, opts...)
	info, err := distributor.client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.
		Info().Str("type", task.Type()).Bytes("payload", task.Payload()).
		Str("queue", info.Queue).Int("max_retry", info.MaxRetry).Msg("enqueued task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskSendOTPEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendOTPEmail
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", asynq.SkipRetry)
	}

	user, err := processor.store.GetUser(payload.Email)
	if err != nil {
		if errors.Is(err, authService.ErrUserNotFound) {
			return fmt.Errorf("user not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	subject := "Welcome to Auth Service"
	otpUrl := fmt.Sprintf("http://localhost:8080/api/v1/otp?code=%s", payload.Token)
	content := fmt.Sprintf(`Hello %s,<br/>
	Click link bellow for login into app!<br/>
	Please <a href="%s">click here</a> to verify your identity.<br/>`, payload.Email, otpUrl)

	to := []string{user.Email}

	err = processor.mailer.SendEmail(subject, content, to, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to send otp email: %w", err)
	}

	log.Info().Str("type", task.Type()).Bytes("payload", task.Payload()).
		Str("email", user.Email).Msg("processed task")
	return nil
}
