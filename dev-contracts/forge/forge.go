package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	scml "github.com/PromptFunctions/promptcontrol/dev-contracts/contracts"
	"github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
)

const (
	defaultMaxRetries = 3
	defaultTimeout    = 90 * time.Second
)

type Config struct {
	MaxRetries int
	Timeout    time.Duration
}

type Client interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string, schema map[string]any) (string, error)
}

type InvalidContractError struct {
	Path    string
	Reason  error
	Missing []string
}

func (e *InvalidContractError) Error() string {
	if e.Reason != nil {
		return e.Reason.Error()
	}
	return "invalid contract"
}

func (e *InvalidContractError) Unwrap() error {
	return e.Reason
}

type RetryError struct {
	reference scml.RenderContract
	Missing   []string
	Retries   int
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("retry limit exhausted after %d attempts", e.Retries)
}

func (e *RetryError) WriteFormattedMissing(w io.Writer) {
	writeFormattedMissing(w, e.reference, e.Missing)
}

func DefaultConfig() Config {
	return Config{
		MaxRetries: defaultMaxRetries,
		Timeout:    defaultTimeout,
	}
}

func RunFile(contractPath string, client Client, cfg Config, trace io.Writer) ([]byte, error) {
	contract, err := scml.ParseFileWithOptions(contractPath, &scml.ParseOptions{
		SkipWriteValidation: true,
	})
	if err != nil {
		ice := &InvalidContractError{
			Path:   contractPath,
			Reason: err,
		}
		var missingErr *scml.MissingWriteTargetsError
		if errors.As(err, &missingErr) && missingErr != nil {
			ice.Missing = append([]string(nil), missingErr.Missing...)
		}
		return nil, ice
	}
	return Run(contract, client, cfg, trace)
}

func Run(contract *scml.Contract, client Client, cfg Config, trace io.Writer) ([]byte, error) {
	if contract == nil {
		return nil, errors.New("contract is required")
	}
	if client == nil {
		return nil, errors.New("client is required")
	}

	cfg = normalizeConfig(cfg)
	reference := contract.RenderView()
	validation := toValidationSpec(contract.RenderValidation())
	schema := contract.RenderSchema()

	systemPrompt, err := buildForgeSystemPrompt(validation)
	if err != nil {
		return nil, err
	}
	referenceShape := buildReferenceShape(reference)
	referenceJSON, err := marshalIndentedJSON(referenceShape)
	if err != nil {
		return nil, err
	}

	writeTraceBlock(trace, "rendered contract", string(referenceJSON))
	writeTraceBlock(trace, "system prompt", systemPrompt)
	expectedKeys := expectedReturnedKeys(reference, validation)

	working := make(map[string]any)
	var missing []string

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		checkpoint := buildAttemptScaffold(reference, working)
		promptScaffold := buildPromptScaffold(reference, checkpoint)
		checkpointJSON, err := marshalIndentedJSON(promptScaffold)
		if err != nil {
			return nil, err
		}

		prompt := buildForgePrompt(string(referenceJSON), string(checkpointJSON), missing)
		writeAttemptStart(trace, attempt, cfg.MaxRetries, missing, len(prompt))
		writeTraceBlock(trace, "user prompt", prompt)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		responseText, err := client.Complete(ctx, systemPrompt, prompt, schema)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("call anthropic: %w", err)
		}
		if trace != nil {
			fmt.Fprintf(trace, "response bytes: %d\n", len(responseText))
		}

		candidate, err := decodeResponse(responseText)
		if err != nil {
			return nil, err
		}
		writeReturnedKeys(trace, candidate, expectedKeys)

		if attempt == 1 {
			working = cloneJSONMap(candidate)
			if trace != nil {
				fmt.Fprintln(trace, "merge: seeded working tree from full response")
			}
		} else {
			merge := mergeMissingValues(working, candidate, checkpoint, missing, validation)
			writeMergeResult(trace, merge)
		}

		checkpoint = buildAttemptScaffold(reference, working)
		result := gating.ValidateContract(checkpoint, reference, validation)
		if result.Status == gating.JSONContractCompletedStatus {
			return json.MarshalIndent(checkpoint, "", "    ")
		}

		missing = append([]string(nil), result.Missing...)
		sort.Strings(missing)
		if trace != nil {
			fmt.Fprintf(trace, "validation missing count: %d\n", len(missing))
			fmt.Fprintf(trace, "retry %d/%d\n", attempt, cfg.MaxRetries)
		}
		writeFormattedMissing(trace, reference, missing)
	}

	return nil, &RetryError{
		reference: reference,
		Missing:   missing,
		Retries:   cfg.MaxRetries,
	}
}

func decodeResponse(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("invalid model JSON: empty response")
	}

	var candidate map[string]any
	if err := json.Unmarshal([]byte(trimmed), &candidate); err != nil {
		return nil, fmt.Errorf("invalid model JSON: %w", err)
	}
	return candidate, nil
}

func toValidationSpec(validation scml.RenderValidation) gating.ValidationSpec {
	spec := gating.ValidationSpec{
		FileLists: make([]gating.FileListSpec, 0, len(validation.FileLists)),
	}
	for _, fileList := range validation.FileLists {
		spec.FileLists = append(spec.FileLists, gating.FileListSpec{
			ItemsPath: fileList.ItemsPath,
			BaseDir:   fileList.BaseDir,
			Mode:      fileList.Mode,
		})
	}
	return spec
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaults.Timeout
	}
	return cfg
}
