package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	apiBase      = "https://api.exerciseapi.dev/v1"
	pageSize     = 100
	pageDeadline = 30 * time.Second
)

// exerciseEnvelope is the shape of GET /v1/exercises responses.
type exerciseEnvelope struct {
	Data   []json.RawMessage `json:"data"`
	Total  *int              `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// categoriesEnvelope is the shape of GET /v1/categories responses.
type categoriesEnvelope struct {
	Data []struct {
		Category    string `json:"category"`
		Count       int    `json:"count"`
		Description string `json:"description"`
	} `json:"data"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	apiKey := os.Getenv("EXERCISE_API_KEY")
	if apiKey == "" {
		log.Fatal("EXERCISE_API_KEY not set")
	}

	// 1. Confirm the strength category count so we can sanity-check the fetch.
	strengthCount, err := confirmStrengthCount(apiKey)
	if err != nil {
		log.Fatalf("confirm strength count: %v", err)
	}
	log.Printf("exerciseapi reports strength count = %d", strengthCount)

	// 2. Paginate GET /v1/exercises?category=strength until we exhaust the set.
	var all []json.RawMessage
	offset := 0
	for {
		url := fmt.Sprintf("%s/exercises?category=strength&limit=%d&offset=%d", apiBase, pageSize, offset)
		env, err := fetchPage(apiKey, url)
		if err != nil {
			log.Fatalf("fetch page offset=%d: %v", offset, err)
		}
		if len(env.Data) == 0 {
			break
		}
		// Inject name_pt: "" into each raw record so the human/translator has a
		// slot to fill in Portuguese without touching any EN field.
		for _, raw := range env.Data {
			injected, err := injectNamePT(raw)
			if err != nil {
				log.Fatalf("inject name_pt: %v", err)
			}
			all = append(all, injected)
		}
		log.Printf("offset=%d fetched %d (cumulative=%d)", offset, len(env.Data), len(all))
		if len(env.Data) < pageSize {
			break
		}
		offset += pageSize
		// Pro tier has no pagination-depth cap, but stop if we blow past a sane
		// safety bound in case the API misbehaves.
		if offset > strengthCount+pageSize {
			log.Fatalf("safety stop: offset %d exceeded expected count %d+%d", offset, strengthCount, pageSize)
		}
	}

	if strengthCount > 0 && len(all) != strengthCount {
		log.Printf("WARNING: fetched %d exercises but API reports strength count = %d", len(all), strengthCount)
	} else {
		log.Printf("count match OK: %d", len(all))
	}

	// 3. Write data/exercises_raw.json — indented for human editing comfort.
	out := struct {
		Source        string            `json:"source"`
		SourceVersion string            `json:"source_version"`
		FetchedAt     time.Time         `json:"fetched_at"`
		Category      string            `json:"category"`
		ExpectedCount int               `json:"expected_count"`
		Count         int               `json:"count"`
		Exercises     []json.RawMessage `json:"exercises"`
	}{
		Source:        "exerciseapi.dev",
		SourceVersion: "v1",
		FetchedAt:     time.Now().UTC(),
		Category:      "strength",
		ExpectedCount: strengthCount,
		Count:         len(all),
		Exercises:     all,
	}

	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("marshal dump: %v", err)
	}
	if err := os.WriteFile("data/exercises_raw.json", append(buf, '\n'), 0o644); err != nil {
		log.Fatalf("write dump: %v", err)
	}
	log.Printf("wrote data/exercises_raw.json (%d exercises)", len(all))
}

func confirmStrengthCount(apiKey string) (int, error) {
	url := apiBase + "/categories"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := (&http.Client{Timeout: pageDeadline}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("categories: status %d: %s", resp.StatusCode, string(body))
	}
	var env categoriesEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return 0, fmt.Errorf("decode categories: %w", err)
	}
	for _, c := range env.Data {
		if c.Category == "strength" {
			return c.Count, nil
		}
	}
	return 0, fmt.Errorf("strength category not found in /v1/categories response")
}

func fetchPage(apiKey, url string) (*exerciseEnvelope, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := (&http.Client{Timeout: pageDeadline}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		ra := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("rate limited (Retry-After=%s)", ra)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	var env exerciseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &env, nil
}

// injectNamePT decodes a raw exercise object as a generic map, sets name_pt to
// "" if absent, and re-encodes it. All original fields are preserved verbatim.
func injectNamePT(raw json.RawMessage) (json.RawMessage, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if _, ok := m["name_pt"]; !ok {
		m["name_pt"] = ""
	}
	return json.Marshal(m)
}