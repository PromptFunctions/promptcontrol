//go:build compliance
// +build compliance

// benchmark_compliance_loop.go
/*
Benchmark: Compliance Loop Convergence (PromptControl vs Naive)

Purpose:
- Compare convergence efficiency between guided contract enforcement (PromptControl) and naive re-prompting.

What it checks:
- Iterations required to reach full contract completion.
- Time to completion across repeated simulations.
- Duplicate key generation (wasted LLM work).
- Completion success rate under bounded iterations.

Simulation model:
- LLM behavior is probabilistic:
  - fills random subsets of keys per iteration (20–40%)
  - may repeat or miss keys
- Two strategies:
  - PromptControl: fills only missing keys each iteration
  - Naive: re-generates entire contract each iteration

Why it matters:
- Demonstrates that PromptControl reduces convergence time and iterations.
- Quantifies reduction in redundant generation (efficiency gain).
- Shows deterministic completion behavior vs probabilistic drift.
- Provides evidence for cost/performance advantages in agent workflows.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PromptFunctions/promptcontrol"
)

const (
	totalKeys = 400
	runs      = 50
	maxIter   = 10
	model     = "gpt-4o-mini"
	maxPreview = 5
)

type Metrics struct {
	iterations int
	keysGen    int
	dupes      int
	completed  bool
	duration   time.Duration
}

var openaiKey = os.Getenv("OPENAI_API_KEY")

func main() {
	contract := generateContract(totalKeys)
	fullKeys := flatten(contract)

	var wg sync.WaitGroup
	var mu sync.Mutex

	pcMetrics := make([]Metrics, 0, runs)
	naiveMetrics := make([]Metrics, 0, runs)

	for i := 0; i < runs; i++ {
		runID := i + 1
		wg.Add(2)

		go func(runID int) {
			defer wg.Done()
			log.Printf("ts=%s benchmark=compliance run=promptcontrol run_id=%d activity=start", nowTS(), runID)
			res := runPromptControl(contract, fullKeys, runID)
			log.Printf(
				"ts=%s benchmark=compliance run=promptcontrol run_id=%d activity=end completed=%t iterations=%d keys_generated=%d dupes=%d duration_ms=%d",
				nowTS(),
				runID,
				res.completed,
				res.iterations,
				res.keysGen,
				res.dupes,
				res.duration.Milliseconds(),
			)
			mu.Lock()
			pcMetrics = append(pcMetrics, res)
			mu.Unlock()
		}(runID)

		go func(runID int) {
			defer wg.Done()
			log.Printf("ts=%s benchmark=compliance run=naive run_id=%d activity=start", nowTS(), runID)
			res := runNaive(contract, fullKeys, runID)
			log.Printf(
				"ts=%s benchmark=compliance run=naive run_id=%d activity=end completed=%t iterations=%d keys_generated=%d dupes=%d duration_ms=%d",
				nowTS(),
				runID,
				res.completed,
				res.iterations,
				res.keysGen,
				res.dupes,
				res.duration.Milliseconds(),
			)
			mu.Lock()
			naiveMetrics = append(naiveMetrics, res)
			mu.Unlock()
		}(runID)
	}

	wg.Wait()
	fmt.Println(buildReport(pcMetrics, naiveMetrics))
}

func runPromptControl(contract map[string]any, fullKeys []string, runID int) Metrics {
	_ = fullKeys
	payload := map[string]any{}
	seen := map[string]struct{}{}
	dupes := 0

	start := time.Now()

	for i := 1; i <= maxIter; i++ {
		status, missing := promptcontrol.JSONContract(payload, contract)
		logValidationActivity("promptcontrol", runID, i, status, missing, len(seen), dupes)
		if status == "completed" {
			return Metrics{i, len(seen), dupes, true, time.Since(start)}
		}

		resp := callLLM(buildPromptMissing(missing))
		keys := extractKeys(resp)

		for _, k := range keys {
			if _, ok := seen[k]; ok {
				dupes++
			}
			seen[k] = struct{}{}
			insert(payload, k)
		}
	}

	return Metrics{maxIter, len(seen), dupes, false, time.Since(start)}
}

func runNaive(contract map[string]any, fullKeys []string, runID int) Metrics {
	payload := map[string]any{}
	seen := map[string]struct{}{}
	dupes := 0

	start := time.Now()

	for i := 1; i <= maxIter; i++ {
		resp := callLLM(buildPromptFull(fullKeys))
		keys := extractKeys(resp)

		for _, k := range keys {
			if _, ok := seen[k]; ok {
				dupes++
			}
			seen[k] = struct{}{}
			insert(payload, k)
		}

		status, missing := promptcontrol.JSONContract(payload, contract)
		logValidationActivity("naive", runID, i, status, missing, len(seen), dupes)
		if status == "completed" {
			return Metrics{i, len(seen), dupes, true, time.Since(start)}
		}
	}

	return Metrics{maxIter, len(seen), dupes, false, time.Since(start)}
}

func logValidationActivity(runType string, runID, iter int, status string, missing []string, keysSeen, dupes int) {
	if status == promptcontrol.JSONContractCompletedStatus {
		log.Printf(
			"ts=%s benchmark=compliance run=%s run_id=%d iter=%d status=%s keys_seen=%d dupes=%d",
			nowTS(),
			runType,
			runID,
			iter,
			status,
			keysSeen,
			dupes,
		)
		return
	}

	log.Printf(
		"ts=%s benchmark=compliance run=%s run_id=%d iter=%d status=%s missing_count=%d missing_preview=%s keys_seen=%d dupes=%d",
		nowTS(),
		runType,
		runID,
		iter,
		status,
		len(missing),
		missingPreview(missing),
		keysSeen,
		dupes,
	)
}

func missingPreview(missing []string) string {
	if len(missing) == 0 {
		return "[]"
	}
	end := maxPreview
	if len(missing) < end {
		end = len(missing)
	}
	return "[" + strings.Join(missing[:end], ",") + "]"
}

func nowTS() string {
	return time.Now().Format(time.RFC3339)
}

func callLLM(prompt string) string {
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	b, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+openaiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)

	choices := out["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)["content"].(string)
	return msg
}

func buildPromptMissing(keys []string) string {
	return fmt.Sprintf("Return ONLY these keys as JSON object, no explanation:\n%s", strings.Join(keys, "\n"))
}

func buildPromptFull(keys []string) string {
	return fmt.Sprintf("Return FULL JSON object containing ALL keys below:\n%s", strings.Join(keys, "\n"))
}

func extractKeys(resp string) []string {
	keys := []string{}
	var m map[string]any
	if err := json.Unmarshal([]byte(resp), &m); err != nil {
		return keys
	}

	var walk func(string, map[string]any)
	walk = func(prefix string, obj map[string]any) {
		for k, v := range obj {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			keys = append(keys, path)
			if child, ok := v.(map[string]any); ok {
				walk(path, child)
			}
		}
	}
	walk("", m)
	return keys
}

func insert(m map[string]any, path string) {
	parts := strings.Split(path, ".")
	curr := m
	for i := 0; i < len(parts); i++ {
		k := parts[i]
		if i == len(parts)-1 {
			curr[k] = "x"
			return
		}
		if _, ok := curr[k]; !ok {
			curr[k] = map[string]any{}
		}
		curr = curr[k].(map[string]any)
	}
}

func generateContract(n int) map[string]any {
	root := map[string]any{}
	for i := 0; i < n; i++ {
		root[fmt.Sprintf("k%d", i)] = map[string]any{
			"a": "",
			"b": "",
			"c": map[string]any{
				"d": "",
				"e": "",
			},
		}
	}
	return root
}

func flatten(m map[string]any) []string {
	out := []string{}
	var walk func(string, map[string]any)

	walk = func(prefix string, obj map[string]any) {
		for k, v := range obj {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			out = append(out, path)
			if child, ok := v.(map[string]any); ok {
				walk(path, child)
			}
		}
	}

	walk("", m)
	sort.Strings(out)
	return out
}

func avgFloat(xs []float64) float64 {
	sum := 0.0
	for _, v := range xs {
		sum += v
	}
	return sum / float64(len(xs))
}

func buildReport(pc, naive []Metrics) string {
	pcIter := []float64{}
	naiveIter := []float64{}
	pcTime := []float64{}
	naiveTime := []float64{}
	pcDup := []float64{}
	naiveDup := []float64{}
	pcSuccess := 0
	naiveSuccess := 0

	for _, m := range pc {
		pcIter = append(pcIter, float64(m.iterations))
		pcTime = append(pcTime, float64(m.duration.Milliseconds()))
		pcDup = append(pcDup, float64(m.dupes))
		if m.completed {
			pcSuccess++
		}
	}

	for _, m := range naive {
		naiveIter = append(naiveIter, float64(m.iterations))
		naiveTime = append(naiveTime, float64(m.duration.Milliseconds()))
		naiveDup = append(naiveDup, float64(m.dupes))
		if m.completed {
			naiveSuccess++
		}
	}

	report := map[string]any{
		"promptcontrol": map[string]any{
			"avg_iterations":      avgFloat(pcIter),
			"avg_completion_time": avgFloat(pcTime),
			"success_rate":        float64(pcSuccess) / float64(len(pc)),
			"duplicate_ratio":     avgFloat(pcDup),
		},
		"naive": map[string]any{
			"avg_iterations":      avgFloat(naiveIter),
			"avg_completion_time": avgFloat(naiveTime),
			"success_rate":        float64(naiveSuccess) / float64(len(naive)),
			"duplicate_ratio":     avgFloat(naiveDup),
		},
		"delta": map[string]any{
			"iteration_gain": avgFloat(naiveIter) - avgFloat(pcIter),
			"time_gain":      avgFloat(naiveTime) - avgFloat(pcTime),
			"efficiency_gain": (avgFloat(naiveIter) - avgFloat(pcIter)) /
				avgFloat(naiveIter),
		},
	}

	b, _ := json.MarshalIndent(report, "", "  ")
	return string(b)
}
