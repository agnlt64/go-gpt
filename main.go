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
	"github.com/atotto/clipboard"
	"github.com/pelletier/go-toml"
)

const (
	CONFIG_FILE = "gpt_config.toml"
)

var (
	history []openai.ChatCompletionMessage
	replCommands = map[string][]string {
		"/help": {},
		"/exit": {},
		"/system": { "show", "reset" },
		"/embed": {},
		"/copy": {},
		"/save": {},
		"/load": {},
	}
)

type Config struct {
	Model string
	RenderMarkdown bool
	SystemPrompt string
	DefaultHistoryPath string
}

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
		fmt.Printf("Error reading `%s`: %v\n", path, err)
		return
	}
	json.Unmarshal(data, &history)
	fmt.Printf("Loaded history from `%s`\n", path)
}

func buildCompleter() *readline.PrefixCompleter {
	pcCommands := []readline.PrefixCompleterInterface{}
	for cmdString := range replCommands {
		pcArgs := []readline.PrefixCompleterInterface{}
		for _, arg := range replCommands[cmdString] {
			pcArgs = append(pcArgs, readline.PcItem(arg))
		}
		pcCommands = append(pcCommands, readline.PcItem(cmdString, pcArgs...))
	}
	return readline.NewPrefixCompleter(pcCommands...)
}

func loadConfig() Config {
	var config Config

	data, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		log.Fatalf("Fatal error: can't load config file: %v", err)
	}

	if err := toml.Unmarshal(data, &config); err != nil {
		log.Fatalf("Fatal error while reading config: %v", err)	
	}

	return config
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)
	running := true

	fmt.Println("GPT Client in Go. Use `/help` for help.")

	config := loadConfig()
	defaultSystemPrompt := config.SystemPrompt
	completer := buildCompleter()

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

	chatResponse := strings.Builder{}

REPL:
	for running {
		line, err := rl.Readline()
		if err != nil {
			break
		}

		if line[0] == '/' {
			commandArgs := []string{}

			for _, str := range strings.Split(line[1:], " ") {
				if str != "" {
					commandArgs = append(commandArgs, str)
				}
			}

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
					config.SystemPrompt += sb.String()
	
					fmt.Printf("Added `%s` to system prompt\n", fileName)
				}
			case "system":
				if len(commandArgs) != 2 {
					fmt.Println("Error: `/system <option>` command expects `show`, or `reset`")
					continue
				}
				switch commandArgs[1] {
				case "show":
					fmt.Println(config.SystemPrompt)
				case "reset":
					config.SystemPrompt = defaultSystemPrompt
					fmt.Println("System prompt has been reset")
				}
			case "save":
				if len(commandArgs) == 1 {
					saveHistory(config.DefaultHistoryPath)
				} else if len(commandArgs) == 2 {
					saveHistory(commandArgs[1])
				} else {
					fmt.Println("Error: `/save <path>` command expects only a file path")
				}
			case "load":
				if len(commandArgs) == 1 {
					loadHistory(config.DefaultHistoryPath)
				} else if len(commandArgs) == 2 {
					loadHistory(commandArgs[1])
				} else {
					fmt.Println("Error: `/load <path>` command expects only a file path")
				}
			case "copy":
				if chatResponse.Len() != 0 {
					err := clipboard.WriteAll(chatResponse.String())
					if err != nil {
						fmt.Printf("Error writing to clipboard: %v", err)
						continue
					}
					fmt.Println("LLM response copied to clipboard")
				} else {
					fmt.Println("Nothing to copy!")
				}
			case "help":
				fmt.Println("Help:")
				fmt.Println("    /system <show | reset> Manipulate the system prompt")
				fmt.Println("    /embed <file>          Embed a file into the system prompt")
				fmt.Println("    /save <path>           Save the history to <path> (JSON format)")
				fmt.Println("    /load <path>           Load the history from <path> (JSON format)")
				fmt.Println("    /copy                  Copy the last LLM response to clipboard")
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
				{Role: openai.ChatMessageRoleSystem, Content: config.SystemPrompt},
			}
			messages = append(messages, history...)
			messages = append(messages, openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleUser,
				Content: line,
			})
			req := openai.ChatCompletionRequest{
				Model: config.Model,
				Messages: messages,
				Stream: true,
			}

			stream, err := client.CreateChatCompletionStream(context.Background(), req)
			if err != nil {
				fmt.Printf("ChatCompletionStream error: %v\n", err)
				return
			}

			chatResponse.Reset()
			for {
				streamResponse, err := stream.Recv()

				if err != nil {
					break
				}
				chunk := streamResponse.Choices[0].Delta.Content
				chatResponse.WriteString(chunk)
				fmt.Print(chunk)
			}
			fullRes := chatResponse.String()
			history = append(history, openai.ChatCompletionMessage{
				Role: "assistant",
				Content: fullRes,
			})
			if config.RenderMarkdown {
				out, _ := glamour.Render(fullRes, "dark")
				fmt.Println("\n--- Rendered Markdown ---")
				fmt.Print(out)
			}
			stream.Close()
			fmt.Println()
		}
	}
}
