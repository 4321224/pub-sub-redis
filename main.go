package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
)

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var ctx = context.Background()
var redisClient = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

func main() {
	defer redisClient.Close()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	app := fiber.New()

	subscriber := redisClient.Subscribe(ctx, "send-user-data")
	defer subscriber.Close()

	go func() {
		user := User{}
		for {
			select {
			case <-stopChan:
				fmt.Println("Shutting down gracefully...")
				return
			default:
				msg, err := subscriber.ReceiveMessage(ctx)
				if err != nil {
					fmt.Println("Error receiving message:", err)
					continue
				}

				if err := json.Unmarshal([]byte(msg.Payload), &user); err != nil {
					fmt.Println("Error unmarshalling message:", err)
					continue
				}

				fmt.Printf("Received user data: %+v\n", user)
			}
		}
	}()

	app.Post("/", func(c *fiber.Ctx) error {
		user := new(User)

		if err := c.BodyParser(user); err != nil {
			fmt.Println("Error parsing request body:", err)
			return c.Status(400).SendString("Bad Request")
		}

		payload, err := json.Marshal(user)
		if err != nil {
			fmt.Println("Error marshalling user:", err)
			return c.Status(500).SendString("Internal Server Error")
		}

		if err := redisClient.Publish(ctx, "send-user-data", payload).Err(); err != nil {
			fmt.Println("Error publishing to Redis:", err)
			return c.Status(500).SendString("Internal Server Error")
		}

		return c.SendStatus(200)
	})

	go func() {
		if err := app.Listen(":8080"); err != nil {
			fmt.Println("Error starting Fiber app:", err)
		}
	}()

	<-stopChan
}
