package main

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	authService "github.com/semmidev/auth-service"
	"github.com/semmidev/auth-service/mail"
	"github.com/semmidev/auth-service/token"
	"github.com/semmidev/auth-service/worker"
	"strings"
)

type AppConfig struct {
	HttpServerAddress   string
	RedisAddr           string
	SecretKey           string
	EmailSenderName     string
	EmailSenderAddress  string
	EmailSenderPassword string
}

func main() {
	appConfig := AppConfig{
		HttpServerAddress:   ":8080",
		RedisAddr:           "0.0.0.0:6379",
		SecretKey:           "12345678901234567890123456789012",
		EmailSenderName:     "yourName",
		EmailSenderAddress:  "yourEmail",
		EmailSenderPassword: "yourAppPassword",
	}

	dataStore := authService.NewMapDataStore()
	tokenMaker, err := token.NewJWTMaker(appConfig.SecretKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create token maker")
	}

	redisOpt := asynq.RedisClientOpt{Addr: "0.0.0.0:6379"}
	taskDistributor := worker.NewRedisTaskDistributor(redisOpt, tokenMaker)
	go runTaskProcessor(appConfig, redisOpt, dataStore)
	runHttpServer(appConfig, dataStore, taskDistributor, tokenMaker)
}

func runHttpServer(
	config AppConfig,
	store authService.UserDataStore,
	taskDistributor worker.TaskDistributor,
	tokenMaker token.Maker,
) {
	app := fiber.New()

	app.Use(logger.New())
	app.Use(cors.New())

	app.Get("/api/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	app.Post("/api/v1/users", func(c *fiber.Ctx) error {
		var signUpInput struct {
			Email string `json:"email"`
		}

		if err := c.BodyParser(&signUpInput); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		newUser, err := authService.NewUser(signUpInput.Email)
		if err != nil {
			if errors.Is(err, authService.ErrorInvalidEmail) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "something went wrong",
			})
		}

		err = store.CreateUser(newUser)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		payload := worker.PayloadSendOTPEmail{
			Email: newUser.Email,
		}

		err = taskDistributor.DistributeTaskSendOTPEmail(context.Background(), &payload)
		if err != nil {
			log.Error().Err(err).Msg("failed to distribute task send otp email")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to distribute task send otp email",
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "please check your email for otp link",
		})
	})

	app.Get("/api/v1/otp", func(c *fiber.Ctx) error {
		code := c.Query("code", "")
		if strings.TrimSpace(code) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid otp code",
			})
		}

		payload, err := tokenMaker.VerifyToken(code)
		if err != nil {
			if errors.Is(err, token.ErrExpiredToken) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "otp code expired",
				})
			}
			if errors.Is(err, token.ErrInvalidToken) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "invalid otp code",
				})
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"data": payload,
		})
	})

	log.Info().Msgf("start http server at %s", config.HttpServerAddress)
	err := app.Listen(config.HttpServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}
}

func runTaskProcessor(config AppConfig, redisOpt asynq.RedisClientOpt, store authService.UserDataStore) {
	mailer := mail.NewGmailSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, mailer)

	log.Info().Msg("start task processor")

	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}
}
