package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

type HTTPRequest struct {
	ctx          context.Context
	debug        bool
	attempt      int
	Method       string
	URL          string
	Path         string
	Organisation string
	Body         []byte
	Error        error
	ResponseInto any
	Headers      map[string]string
	Query        map[string]string
	client       *http.Client
	retry        func(int, *http.Response) (int, bool)
}

type HTTPBuilder struct {
	client *http.Client
	URL    string
}

func NewHTTPBuilder(url string) *HTTPBuilder {
	return &HTTPBuilder{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
		URL: url,
	}
}

func (h *HTTPBuilder) New() *HTTPRequest {
	req := &HTTPRequest{
		client: h.client,
		URL:    h.URL,
	}

	if os.Getenv("HTTP_DEBUG") == "true" || os.Getenv("HTTP_DEBUG") == "True" {
		req.debug = true
	}

	return req
}

func (h *HTTPRequest) WithQueryParam(key, value string) *HTTPRequest {
	if h.Query == nil {
		h.Query = make(map[string]string)
	}
	h.Query[key] = value

	return h
}

func (h *HTTPRequest) WithMethod(method string) *HTTPRequest {
	h.Method = method

	return h
}

func (h *HTTPRequest) WithPath(path string) *HTTPRequest {
	h.Path = path

	return h
}

func (h *HTTPRequest) WithOrganisation(organisation string) *HTTPRequest {
	h.Organisation = organisation

	return h
}

func (h *HTTPRequest) WithJWT(jwt string) *HTTPRequest {
	return h.WithHeader("Authorization", "JWT "+jwt)
}

func (h *HTTPRequest) WithContext(ctx context.Context) *HTTPRequest {
	h.ctx = ctx

	return h
}

// WithRetry sets the retry function for the request. The retry function is called with the attempt number and the response.
// The body is not available on the response and is already read and discarded.
// The function can return an false to stop the retry loop or return the number of seconds to wait before retrying.
func (h *HTTPRequest) WithRetry(retry func(int, *http.Response) (int, bool)) *HTTPRequest {
	h.retry = retry

	return h
}

func (h *HTTPRequest) JSONBody(b any) *HTTPRequest {
	bytes, err := json.Marshal(b)
	if err != nil {
		h.Error = err

		return h
	}

	h.Body = bytes

	return h.WithHeader("Content-Type", "application/json")
}

func (h *HTTPRequest) WithHeader(key, value string) *HTTPRequest {
	if h.Headers == nil {
		h.Headers = make(map[string]string)
	}

	h.Headers[key] = value

	return h
}

func (h *HTTPRequest) Into(v any) *HTTPRequest {
	h.ResponseInto = v

	return h
}

func (h *HTTPRequest) Do() error {
	if h.Error != nil {
		return h.Error
	}

	if h.URL == "" {
		return errors.New("url is empty")
	}

	method := h.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if len(h.Body) > 0 {
		body = bytes.NewReader(h.Body)
	}

	ctx := h.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	fullUrl, err := url.JoinPath(h.URL, h.Path)
	if err != nil {
		return fmt.Errorf("failed to join url: %w", err)
	}

	fullUrl = strings.ReplaceAll(fullUrl, ":organisation", h.Organisation)

	// attach query params if any
	if len(h.Query) > 0 {
		u, err := url.Parse(fullUrl)
		if err != nil {
			return fmt.Errorf("failed to parse url: %w", err)
		}
		q := u.Query()
		for k, v := range h.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		fullUrl = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullUrl, body)
	if err != nil {
		return err
	}

	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}
	if h.ResponseInto != nil && req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	if h.debug {
		reqBytes, _ := httputil.DumpRequest(req, true)
		fmt.Fprintln(os.Stderr, string(reqBytes))
	}

	h.attempt++

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if h.debug {
		respBytes, _ := httputil.DumpResponse(resp, true)
		fmt.Fprintln(os.Stderr, string(respBytes))
	}

	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)

		if h.retry != nil {
			after, shouldRetry := h.retry(h.attempt, resp)
			if shouldRetry {
				// sleep for the specified time
				time.Sleep(time.Duration(after) * time.Second)

				return h.Do()
			}
		}

		var apiErr APIError
		jsonErr := json.Unmarshal(b, &apiErr)
		if jsonErr == nil && apiErr.Message != "" {
			return &apiErr
		}

		msg := strings.TrimSpace(string(b))
		if msg != "" {
			return fmt.Errorf("http %s %s: status %d: %s", method, h.URL, resp.StatusCode, msg)
		}

		return fmt.Errorf("http %s %s: status %d", method, h.URL, resp.StatusCode)
	}

	if h.ResponseInto == nil {
		_, _ = io.Copy(io.Discard, resp.Body)

		return nil
	}

	switch v := h.ResponseInto.(type) {
	case *[]byte:
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		*v = b

		return nil
	case *string:
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		*v = string(b)

		return nil
	default:
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if err := json.Unmarshal(bodyBytes, h.ResponseInto); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("failed to decode response: %w", err)
		}

		return nil
	}
}
