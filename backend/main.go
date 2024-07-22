package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func main() {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173/",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Use(logger.New())

	app.Post("/chat/", chatHandler)

	log.Fatal(app.Listen(":8000"))
}

func chatHandler(c *fiber.Ctx) error {
	log.Println("Received request for chat")

	apiKey := os.Getenv("NVIDIA_API_KEY")
	apiURL := "https://integrate.api.nvidia.com/v1/chat/completions"

	var requestData map[string]interface{}

	// Parse body from request into JSON
	if err := c.BodyParser(&requestData); err != nil {
		log.Printf("Error parsing request body: %v\n", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	question, ok := requestData["question"].(string)
	if !ok || question == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid question format or empty question",
		})
	}

	requestPayload := map[string]interface{}{
		"model": "meta/llama3-70b-instruct",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an AI that provides direct answers to coding questions.",
			},
			{
				"role":    "user",
				"content": question,
			},
		},
		"temperature": 0.5,
		"top_p":       1,
		"max_tokens":  1024,
	}

	jsonValue, _ := json.Marshal(requestPayload)
	log.Printf("Sending request to NVIDIA NIM API: %s\n", string(jsonValue))

	// Create a new HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error creating request: %v", err),
		})
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v\n", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error sending request: %v", err),
		})
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v\n", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error reading response body: %v", err),
		})
	}

	log.Printf("Response status: %s\n", resp.Status)
	log.Printf("Response body: %s\n", string(body))

	// If the status is not 200 OK, return an error
	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"error": fmt.Sprintf("API returned non-200 status: %s\nBody: %s", resp.Status, string(body)),
		})
	}

	// If we got here, we have a 200 OK response
	// Parse the response JSON
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error parsing JSON response: %v\n", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Error parsing JSON response: %v", err),
		})
	}

	// Extract the answer from the response
	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Unexpected response structure from API",
		})
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Unexpected response structure from API",
		})
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Unexpected response structure from API",
		})
	}

	answer, ok := message["content"].(string)
	if !ok {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Unexpected response structure from API",
		})
	}

	return c.JSON(fiber.Map{
		"answer": answer,
	})
}
