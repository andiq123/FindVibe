package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Google's latin-1 suggest feed uses ã/Ã where Romanian wants ă/Ă.
var roSuggestFix = strings.NewReplacer("ã", "ă", "Ã", "Ă")

type SuggestionsService struct {
	client *http.Client
}

func NewSuggestionsService(client *http.Client) *SuggestionsService {
	return &SuggestionsService{client: client}
}

func (ss *SuggestionsService) GetSuggestions(ctx context.Context, query, hl, gl string) ([]string, error) {
	hl = localeCode(hl, "en")
	gl = strings.ToUpper(localeCode(gl, "US"))

	apiURL := fmt.Sprintf(
		"https://suggestqueries.google.com/complete/search?client=firefox&hl=%s&gl=%s&q=%s",
		url.QueryEscape(hl),
		url.QueryEscape(gl),
		url.QueryEscape(query),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("suggestions: failed to create request: %w", err)
	}

	resp, err := ss.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("suggestions: failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("suggestions: unexpected status code: %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("suggestions: failed to read body: %w", err)
	}

	start := bytes.IndexByte(payload, '[')
	end := bytes.LastIndexByte(payload, ']')
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("suggestions: invalid format in response")
	}

	// Google emits ISO-8859-1; encoding/json expects UTF-8.
	raw := latin1ToUTF8(payload[start : end+1])

	var data []any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("suggestions: unmarshal error: %w", err)
	}
	if len(data) < 2 {
		return nil, errors.New("suggestions: empty data in response")
	}

	items, ok := data[1].([]any)
	if !ok {
		return nil, errors.New("suggestions: failed to cast suggestions list")
	}

	fixRO := hl == "ro" || gl == "RO"
	results := make([]string, 0, len(items))
	for _, rawItem := range items {
		text := suggestionText(rawItem)
		if text == "" {
			continue
		}
		if fixRO {
			text = roSuggestFix.Replace(text)
		}
		results = append(results, text)
	}
	return results, nil
}

func suggestionText(rawItem any) string {
	switch v := rawItem.(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if text, ok := v[0].(string); ok {
				return text
			}
		}
	}
	return ""
}

// localeCode keeps a 2-letter alpha tag; drops junk from the client.
func localeCode(s, fallback string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return fallback
	}
	a, b := s[0]|0x20, s[1]|0x20 // ASCII lower
	if a < 'a' || a > 'z' || b < 'a' || b > 'z' {
		return fallback
	}
	return string([]byte{a, b})
}

// latin1ToUTF8 maps each byte to a Unicode code point (ISO-8859-1).
func latin1ToUTF8(b []byte) []byte {
	out := make([]byte, 0, len(b)+len(b)/4)
	for _, c := range b {
		out = utf8.AppendRune(out, rune(c))
	}
	return out
}
