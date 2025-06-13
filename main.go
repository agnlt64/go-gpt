package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
	"github.com/chzyer/readline"
	"github.com/charmbracelet/glamour"
)

const (
	DEFAULT_SYSTEM_PROMPT = "You are a terminal-based chat assistant. Give relatively short answers, while being as accurate as possible."
	DEFAULT_HISTORY_PATH = "history.json"
)

var (
	history []openai.ChatCompletionMessage
)

func saveHistory(path string) {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		fmt.Printf("Error saving `%s`: %v", path, err)
		return
	}
	os.WriteFile(path, data, 0644)
	fmt.Printf("History saved to `%s`\n", path)
}

func loadHistory(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading `%s`: %v", path, err)
		return
	}
	json.Unmarshal(data, &history)
	fmt.Printf("Loaded history from `%s`\n", path)
}

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

	completer := readline.NewPrefixCompleter(
		readline.PcItem("/help"),
		readline.PcItem("/exit"),
		readline.PcItem("/embed"),
		readline.PcItem("/system",
			readline.PcItem("show"),
			readline.PcItem("reset"),
		),
		readline.PcItem("/save"),
		readline.PcItem("/load"),
	)

	rl, err := readline.NewEx(&readline.Config{
		Prompt: ">",
		AutoComplete: completer,
		HistoryFile: "/tmp/gpt_repl_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt: "exit",
	})
	if err != nil {
		log.Fatalf("readline error: %v", err)
	}
	defer rl.Close()

REPL:
	for running {
		line, err := rl.Readline()
		if err != nil {
			break
		}

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
			case "save":
				if len(commandArgs) == 1 {
					saveHistory(DEFAULT_HISTORY_PATH)
				} else if len(commandArgs) == 2 {
					saveHistory(commandArgs[1])
				} else {
					fmt.Println("Error: `/save <path>` command expects only a file path")
				}
			case "load":
				if len(commandArgs) == 1 {
					loadHistory(DEFAULT_HISTORY_PATH)
				} else if len(commandArgs) == 2 {
					loadHistory(commandArgs[1])
				} else {
					fmt.Println("Error: `/load <path>` command expects only a file path")
				}
			case "help":
				fmt.Println("Help:")
				fmt.Println("    /system <show | reset> Manipulate the system prompt")
				fmt.Println("    /embed <file>          Embed a file into the system prompt")
				fmt.Println("    /help                  Display this help")
				fmt.Println("    /exit                  Exit the REPL")
			default:
				fmt.Printf("Error: `%s` is not a valid REPL command\n", commandArgs[0])
			}
		} else {
			history = append(history, openai.ChatCompletionMessage{
				Role: "user",
				Content: line,
			})
			messages := []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			}
			messages = append(messages, history...)
			messages = append(messages, openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleUser,
				Content: line,
			})
			req := openai.ChatCompletionRequest{
				Model: openai.GPT4oMini,
				Messages: messages,
				Stream: true,
			}

			stream, err := client.CreateChatCompletionStream(context.Background(), req)
			if err != nil {
				fmt.Printf("ChatCompletionStream error: %v\n", err)
				return
			}

			chatResponse := strings.Builder{}
			for {
				streamResponse, err := stream.Recv()

				if err != nil {
					break
				}
				chunk := streamResponse.Choices[0].Delta.Content
				chatResponse.WriteString(chunk)
				fmt.Print(chunk)
				// TODO: allow to copy the response to clipboard
				// github.com/atotto/clipboard
			}
			fullRes := chatResponse.String()
			history = append(history, openai.ChatCompletionMessage{
				Role: "assistant",
				Content: fullRes,
			})
			out, _ := glamour.Render(fullRes, "dark")
			fmt.Println("\n--- Rendered Markdown ---")
			fmt.Print(out)
			stream.Close()
			fmt.Println()
		}
	}
}
