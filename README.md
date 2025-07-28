# Go GPT

A terminal-based ChatGPT client written in Go.

## Quick Start
You'll need an OpenAI API key, You can get one [here](https://platform.openai.com/). Put it in `.env`, and then run:
```console
$ go mod tidy
$ go run .
```
Use the `/help` for all the available commands.

## Config file
Your config must be in `gpt_config.toml`. Here is an example:
```python
Model = "gpt-4o-mini"
RenderMarkdown = true
DefaultHistoryPath = "history.json"
SystemPrompt = "You are a terminal-based chat assistant. Give relatively short answers, while being as accurate as possible."
CommandPrefix = "/"
Theme = "dark"
```
`CommandPrefix` must be a single character. `Theme` should be a valid [glamour](https://github.com/charmbracelet/glamour/tree/master/styles/gallery) theme.