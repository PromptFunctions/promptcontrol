//go:build !compliance && !large
// +build !compliance,!large

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	promptcontrol "github.com/PromptFunctions/promptcontrol/dev-contracts/forge"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultAnthropicAPIKey = "sk-ant-..."
	defaultAnthropicModel  = "claude-haiku-4-5"
	defaultMaxTokens       = int64(4096)
)

type anthropicConfig struct {
	Model     string
	MaxTokens int64
	Forge     promptcontrol.Config
}

type anthropicClient struct {
	client    anthropic.Client
	model     string
	maxTokens int64
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: go run ./benchmark <contract-path>")
		return 1
	}

	contractPath := args[0]

	cfg, err := loadAnthropicConfig()
	if err != nil {
		writeForgeError(stderr, err)
		return 1
	}

	client, err := newAnthropicClient(cfg)
	if err != nil {
		writeForgeError(stderr, err)
		return 1
	}

	output, err := promptcontrol.RunFile(contractPath, client, cfg.Forge, stderr)
	if err != nil {
		var ice *promptcontrol.InvalidContractError
		if errors.As(err, &ice) {
			writeInvalidContract(stderr, ice)
			return 1
		}
		writeForgeError(stderr, err)
		return 1
	}

	if _, err := stdout.Write(output); err != nil {
		writeForgeError(stderr, err)
		return 1
	}
	return 0
}

func (c anthropicClient) Complete(ctx context.Context, systemPrompt, userPrompt string, schema map[string]any) (string, error) {
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: c.maxTokens,
		System: []anthropic.TextBlockParam{{
			Text: systemPrompt,
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: schema,
			},
		},
	})
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, block := range msg.Content {
		if block.Type != "text" {
			continue
		}
		builder.WriteString(block.AsText().Text)
	}
	if builder.Len() == 0 {
		return "", errors.New("anthropic response did not contain JSON text")
	}
	return builder.String(), nil
}

func loadAnthropicConfig() (anthropicConfig, error) {
	cfg := anthropicConfig{
		Model:     defaultAnthropicModel,
		MaxTokens: defaultMaxTokens,
		Forge:     promptcontrol.DefaultConfig(),
	}

	if model := strings.TrimSpace(os.Getenv("ANTHROPIC_MODEL")); model != "" {
		cfg.Model = model
	}

	maxTokens, err := parseEnvInt64("ANTHROPIC_MAX_TOKENS", defaultMaxTokens)
	if err != nil {
		return anthropicConfig{}, err
	}
	cfg.MaxTokens = maxTokens

	maxRetries, err := parseEnvInt("ANTHROPIC_MAX_RETRIES", cfg.Forge.MaxRetries)
	if err != nil {
		return anthropicConfig{}, err
	}
	cfg.Forge.MaxRetries = maxRetries

	return cfg, nil
}

func parseEnvInt(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func parseEnvInt64(name string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func newAnthropicClient(cfg anthropicConfig) (promptcontrol.Client, error) {
	apiKey, err := resolveAnthropicAPIKey()
	if err != nil {
		return nil, err
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return anthropicClient{
		client:    client,
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
	}, nil
}

func resolveAnthropicAPIKey() (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(defaultAnthropicAPIKey)
	}
	if apiKey == "" {
		return "", errors.New("ANTHROPIC_API_KEY is required")
	}
	return apiKey, nil
}

func writeInvalidContract(stderr io.Writer, ice *promptcontrol.InvalidContractError) {
	fmt.Fprintln(stderr, "invalid contract")
	fmt.Fprintf(stderr, "path: %s\n", ice.Path)
	fmt.Fprintf(stderr, "reason: %s\n", formatContractError(ice.Reason))
	if len(ice.Missing) > 0 {
		fmt.Fprintf(stderr, "missing: %s\n", formatStringList(ice.Missing))
	}
}

func writeForgeError(stderr io.Writer, err error) {
	fmt.Fprintln(stderr, "forge failed")
	fmt.Fprintf(stderr, "reason: %s\n", err.Error())

	var retryErr *promptcontrol.RetryError
	if errors.As(err, &retryErr) && retryErr != nil && len(retryErr.Missing) > 0 {
		retryErr.WriteFormattedMissing(stderr)
	}
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}

	var builder strings.Builder
	builder.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			builder.WriteString(", ")
		}
		fmt.Fprintf(&builder, "%q", value)
	}
	builder.WriteByte(']')
	return builder.String()
}

func formatContractError(err error) string {
	if !shouldShowSCMLHint(err) {
		return err.Error()
	}
	return err.Error() + ". Contract elements should be wrapped around with <scml> </scml> keys"
}

func shouldShowSCMLHint(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, marker := range []string{
		"raw XML-like syntax must be wrapped in HTML comments",
		"invalid SCML comment marker",
		"unexpected root element <",
		"parse normalized SCML document",
		"non-whitespace content before <scml>",
		"non-whitespace content inside <scml>",
		"unknown XML element <",
		"unknown XML attribute ",
		"<scml> block",
		"<section> blocks must appear",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
