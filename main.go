package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/glamour"
	"github.com/chzyer/readline"
	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml"
	"github.com/sashabaranov/go-openai"
)

const (
	CONFIG_FILE = "gpt_config.toml"
)

var (
	history      []openai.ChatCompletionMessage
	// TODO: add shortcuts (/q, /h, ...)
	// TODO: add /summary
	// TODO: add /export html | md
	replCommands = []Command{
		NewCommand("system", []string{"show", "reset"}, "Manipulate the system prompt"),
		NewCommand("embed", []string{"file"}, "Embed a file into the system prompt"),
		NewCommand("save", []string{"path"}, "Save the history to <path> (JSON format)"),
		NewCommand("load", []string{"path"}, "Load the history from <path> (JSON format)"),
		NewCommand("copy", []string{}, "Copy the last LLM response to clipboard"),
		NewCommand("config", []string{}, "Show / edit the config"),
		NewCommand("help", []string{}, "Display this help"),
		NewCommand("exit", []string{}, "Exit the REPL"),
	}
)

type Config struct {
	Model              string
	RenderMarkdown     bool
	Theme              string
	SystemPrompt       string
	DefaultHistoryPath string
	CommandPrefix      string
}

type Command struct {
	Name        string
	Args        []string
	Description string
}

func NewCommand(name string, args []string, desc string) Command {
	return Command{
		Name:        name,
		Args:        args,
		Description: desc,
	}
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

func buildCompleter(prefix string) *readline.PrefixCompleter {
	pcCommands := []readline.PrefixCompleterInterface{}
	for _, cmd := range replCommands {
		pcArgs := []readline.PrefixCompleterInterface{}
		for _, arg := range cmd.Args {
			pcArgs = append(pcArgs, readline.PcItem(arg))
		}
		pcCommands = append(pcCommands, readline.PcItem(prefix+cmd.Name, pcArgs...))
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

func saveConfig(config Config) {
	data, err := toml.Marshal(config)
	if err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		return
	}
	err = os.WriteFile(CONFIG_FILE, data, 0644)
	if err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}
}

func printCommandHelp(commands []Command, prefix string) {
	commandStrings := buildCommandStrings(commands, prefix)
	maxLength := findMaxLength(commandStrings)
	fmt.Println("Help:")
	printFormattedHelp(commands, commandStrings, maxLength)
}

func buildCommandStrings(commands []Command, prefix string) []string {
	var result []string
	for _, cmd := range commands {
		sb := strings.Builder{}
		sb.WriteString(fmt.Sprintf("    %s%s ", prefix, cmd.Name))

		if len(cmd.Args) > 0 {
			sb.WriteString("<")
			sb.WriteString(strings.Join(cmd.Args, " | "))
			sb.WriteString(">")
		}

		result = append(result, sb.String())
	}
	return result
}

func findMaxLength(strings []string) int {
	maxLen := 0
	for _, s := range strings {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	return maxLen
}

func printFormattedHelp(commands []Command, cmdStrings []string, maxLen int) {
	for i, cmd := range commands {
		fmt.Printf("%s", cmdStrings[i])
		padding := maxLen - len(cmdStrings[i]) + 1
		fmt.Printf("%*s%s\n", padding, "", cmd.Description)
	}
}

func printConfig(config Config) {
	data, err := toml.Marshal(config)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)
	running := true

	config := loadConfig()
	fmt.Printf("GPT Client in Go. Use `%shelp` for help.\n", config.CommandPrefix)

	defaultSystemPrompt := config.SystemPrompt
	completer := buildCompleter(config.CommandPrefix)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ">",
		AutoComplete:    completer,
		HistoryFile:     "/tmp/gpt_repl_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
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

		if line[0] == []byte(config.CommandPrefix)[0] {
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
					fmt.Printf("Error: `%sembed <file>` command expects at least a file name\n", config.CommandPrefix)
					continue
				}

				for idx := range len(commandArgs) - 1 {
					fileName := commandArgs[idx+1]
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
					fmt.Printf("Error: `%ssystem <option>` command expects `show`, or `reset`\n", config.CommandPrefix)
					continue
				}
				switch commandArgs[1] {
				case "show":
					fmt.Println(config.SystemPrompt)
				case "reset":
					config.SystemPrompt = defaultSystemPrompt
					fmt.Println("System prompt has been reset")
				}
			case "save", "load":
				// TODO: autocomplete file path
				// TODO: underline file names
				action := commandArgs[0]
				if len(commandArgs) > 2 {
					fmt.Printf("Error: `%s%s <path>` command expects only a file path\n", config.CommandPrefix, action)
					continue
				}

				path := config.DefaultHistoryPath
				if len(commandArgs) == 2 {
					path = commandArgs[1]
				}

				if action == "save" {
					saveHistory(path)
				} else {
					loadHistory(path)
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
			case "config":
				if len(commandArgs) == 1 {
					printConfig(config)
					continue
				}
				if len(commandArgs) != 3 {
					fmt.Printf("Usage: %sconfig <Field> <Value>\n", config.CommandPrefix)
					continue
				}
				field := commandArgs[1]
				value := commandArgs[2]
				updated := false
				// Use reflection to set the field
				cfgVal := reflect.ValueOf(&config).Elem()
				cfgType := cfgVal.Type()
				for i := range cfgVal.NumField() {
					f := cfgType.Field(i)
					if strings.EqualFold(f.Name, field) {
						fieldVal := cfgVal.Field(i)
						switch fieldVal.Kind() {
						case reflect.Bool:
							b := strings.EqualFold(value, "true") || value == "1"
							fieldVal.SetBool(b)
							updated = true
						case reflect.String:
							fieldVal.SetString(value)
							updated = true
						default:
							fmt.Printf("Unsupported config field type: %s\n", f.Type.Name())
						}
						break
					}
				}
				if updated {
					saveConfig(config)
					fmt.Printf("Config updated: %s = %s\n", field, value)
				} else {
					fmt.Printf("Unknown config field: %s\n", field)
				}
			case "help":
				printCommandHelp(replCommands, config.CommandPrefix)
			default:
				fmt.Printf("Error: `%s` is not a valid REPL command\n", commandArgs[0])
			}
		} else {
			history = append(history, openai.ChatCompletionMessage{
				Role:    "user",
				Content: line,
			})
			messages := []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: config.SystemPrompt},
			}
			messages = append(messages, history...)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: line,
			})
			req := openai.ChatCompletionRequest{
				Model:    config.Model,
				Messages: messages,
				Stream:   true,
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
				Role:    "assistant",
				Content: fullRes,
			})
			if config.RenderMarkdown {
				out, _ := glamour.Render(fullRes, config.Theme)
				fmt.Println("\n--- Rendered Markdown ---")
				fmt.Print(out)
			}
			stream.Close()
			fmt.Println()
		}
	}
}
