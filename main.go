package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

const (
	DEFAULT_SYSTEM_PROMPT = "You are a terminal-based chat assistant. Give relatively short answers, while being as accurate as possible."
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

	fmt.Println("GPT Client in Go. Use `/help` for help.")

REPL:
	for running {
		// TODO: add better prompt with readline
		// github.com/chzyer/readline 
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
				if len(commandArgs) < 2 {
					fmt.Println("Error: `/embed <file>` command expects at least a file name")
					continue
				}

				for idx := range len(commandArgs) - 1 {
					fileName := commandArgs[idx + 1]
					content, err := os.ReadFile(fileName)
					if err != nil {
						fmt.Printf("Error: can't read file `%s`\n", fileName)
						continue
					}
	
					sb := strings.Builder{}
					sb.WriteString(fmt.Sprintf("\nFile `%s`:\n", fileName))
					sb.Write(content)
					systemPrompt += sb.String()
	
					fmt.Printf("Added `%s` to system prompt\n", fileName)
				}
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
					fmt.Println("System prompt has been reset")
				}
			case "help":
				fmt.Println("Help:")
				fmt.Println("    /system <show | reset> Manipulate the system prompt")
				fmt.Println("    /embed <file>          Embed a file into the system prompt")
				fmt.Println("    /help                  Display this help")
				fmt.Println("    /exit                  Exit the REPL")
				// TODO: add /save and /load commands to save and load the history (JSON)
			default:
				fmt.Printf("Error: `%s` is not a valid REPL command\n", commandArgs[0])
			}
		} else {
			req := openai.ChatCompletionRequest{
				Model: openai.GPT4oMini,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
					{Role: openai.ChatMessageRoleAssistant, Content: line},
				},
				Stream: true,
			}

			stream, err := client.CreateChatCompletionStream(context.Background(), req)
			if err != nil {
				fmt.Printf("ChatCompletionStream error: %v\n", err)
				return
			}

			for {
				response, err := stream.Recv()

				if err != nil {
					break
				}

				// TODO: add colored output
				// github.com/fatih/color or github.com/mgutz/ansi
				// TODO: render markdown
				// github.com/charmbracelet/glamour
				fmt.Printf(response.Choices[0].Delta.Content)
				// TODO: allow to copy the response to clipboard
				// github.com/atotto/clipboard
			}
			stream.Close()
			fmt.Println()
		}
	}
}
