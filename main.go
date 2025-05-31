package main

import (
	"context"
	"fmt"
	"log"
	"bufio"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/joho/godotenv"
)

const (
	DEFAULT_SYSTEM_PROMPT = "You are a terminal-based chat assistant. Give relatively short answers, while being as accurate as possible.\n"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)
	running := true
	systemPrompt := DEFAULT_SYSTEM_PROMPT

	fmt.Println("GPT Client in Go")
	REPL: for running {
		fmt.Print("> ")
		input := bufio.NewReader(os.Stdin)
		line, err := input.ReadString('\n')

		if err != nil {
			log.Fatalf("Erreur de lecture de l'entrée: %v", err)
		}
		
		line = strings.TrimSuffix(line, "\n")

		if line[0] == '/' {
			commandArgs := strings.Split(line[1:], " ")
			switch commandArgs[0] {
			case "exit":
				running = false
				fmt.Println("Goodbye!")
				break REPL
			case "embed":
				if len(commandArgs) != 2 {
					fmt.Println("Error: `/embed <file>` command expects a single file name")
					continue
				}
				fileName := commandArgs[1]
				// TODO: extract file content and add it to system prompt.
				content, err := os.ReadFile(fileName)
				if err != nil {
					fmt.Printf("Error: can't read file `%s`\n", fileName)
					continue
				}

				sb := strings.Builder{}
				sb.WriteString(fmt.Sprintf("File `%s`:\n", fileName))
				sb.Write(content)
				systemPrompt += sb.String()

				fmt.Printf("Added `%s` to system prompt\n", fileName)
			case "system":
				if len(commandArgs) != 2 {
					fmt.Println("Error: `/system <option>` command expects `show`, or `reset`")
					continue
				}
				switch commandArgs[1] {
				case "show":
					fmt.Println(systemPrompt)
				case "reset":
					systemPrompt = DEFAULT_SYSTEM_PROMPT
				}
			default:
				fmt.Printf("Error: `%s` is not a valid REPL command\n", commandArgs[0])
			}
		} else {
			resp, err := client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model: openai.GPT4oMini,
					Messages: []openai.ChatCompletionMessage{
						{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
						{Role: openai.ChatMessageRoleAssistant, Content: line},
					},
				},
			)
	
			if err != nil {
				log.Fatalf("Erreur API: %v", err)
			}
	
			fmt.Println(resp.Choices[0].Message.Content)
		}
	}
}
