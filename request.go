package invoq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type requestClientOptions struct {
	apiKey     string
	apiOrigin  *url.URL
	httpClient *http.Client
	timeout    time.Duration
}

func requestJSON[T any](
	ctx context.Context,
	clientOptions requestClientOptions,
	method string,
	pathSegments []string,
	body any,
) (*T, error) {
	requestURL := buildRequestURL(clientOptions.apiOrigin, pathSegments)

	var requestBody io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewReader(bodyBytes)
	}

	requestContext, cancel := context.WithTimeout(ctx, clientOptions.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestContext, method, requestURL.String(), requestBody)
	if err != nil {
		return nil, wrapConnectError(err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+clientOptions.apiKey)
	request.Header.Set("User-Agent", userAgent())
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := clientOptions.httpClient.Do(request)
	if err != nil {
		if isTimeoutError(requestContext, err) {
			return nil, wrapTimeoutError(err)
		}

		return nil, wrapConnectError(err)
	}
	defer response.Body.Close()

	responseBytes, err := io.ReadAll(response.Body)
	if err != nil {
		if isTimeoutError(requestContext, err) {
			return nil, wrapTimeoutError(err)
		}

		return nil, wrapResponseReadError(err)
	}

	payload, err := decodeJSON(responseBytes)
	if err != nil {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, newAPIError(response.StatusCode, string(responseBytes))
		}

		return nil, wrapParseResponseError(err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, newAPIError(response.StatusCode, payload)
	}

	envelope, ok := payload.(map[string]any)
	if !ok {
		return nil, invalidResponseError("invoq API response did not include a data envelope.", nil, payload)
	}

	data, ok := envelope["data"]
	if !ok {
		return nil, invalidResponseError("invoq API response did not include a data envelope.", nil, payload)
	}
	// A non-object data is a broken envelope, not a resource that failed to parse.
	if _, isObject := data.(map[string]any); !isObject {
		return nil, invalidResponseError("invoq API response data envelope was not an object.", nil, payload)
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, unexpectedMarshalError(err)
	}

	var result T
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, wrapParseResponseError(err)
	}

	return &result, nil
}

func isTimeoutError(ctx context.Context, err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return true
	}

	var netError net.Error
	return errors.As(err, &netError) && netError.Timeout()
}

func decodeJSON(value []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()

	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid JSON")
		}

		return nil, err
	}

	return payload, nil
}

// PathEscape leaves `&`, `=` and `+` unescaped — RFC 3986-legal, but the JS and
// other four SDKs escape them. Unreachable today: ids are Crockford base32.
func buildRequestURL(baseURL *url.URL, pathSegments []string) *url.URL {
	requestURL := *baseURL

	escapedSegments := make([]string, 0, len(pathSegments))
	decodedSegments := make([]string, 0, len(pathSegments))
	for _, segment := range pathSegments {
		escapedSegments = append(escapedSegments, url.PathEscape(segment))
		decodedSegments = append(decodedSegments, segment)
	}

	requestURL.Path = "/" + strings.Join(decodedSegments, "/")
	requestURL.RawPath = "/" + strings.Join(escapedSegments, "/")

	return &requestURL
}
