package invoq

// SDKError is implemented by errors returned by this SDK.
type SDKError interface {
	error
	invoqSDKError()
}

// Error is a top-level SDK error for configuration, connection, and response failures.
type Error struct {
	Message string
	Err     error
	Payload any
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func (e *Error) invoqSDKError() {}

func configurationError(message string) *Error {
	return &Error{Message: message}
}

// APIError is returned when the invoq API responds with a non-2xx status.
type APIError struct {
	Message string
	Status  int
	Code    string
	Fields  []APIErrorField
	Meta    map[string]any
	Payload any
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}

	return e.Message
}

func (e *APIError) invoqSDKError() {}

// SignatureVerificationErrorCode is a stable webhook verification error code.
type SignatureVerificationErrorCode string

const (
	// SignatureErrorMissingSignature means the invoq-signature header was absent.
	SignatureErrorMissingSignature SignatureVerificationErrorCode = "missing_signature"
	// SignatureErrorInvalidSignatureHeader means the invoq-signature header was malformed.
	SignatureErrorInvalidSignatureHeader SignatureVerificationErrorCode = "invalid_signature_header"
	// SignatureErrorTimestampOutsideTolerance means the webhook timestamp was too old or too far in the future.
	SignatureErrorTimestampOutsideTolerance SignatureVerificationErrorCode = "timestamp_outside_tolerance"
	// SignatureErrorSignatureMismatch means the expected HMAC did not match the header signature.
	SignatureErrorSignatureMismatch SignatureVerificationErrorCode = "signature_mismatch"
	// SignatureErrorInvalidPayload means the signed body was not a valid invoq webhook payload.
	SignatureErrorInvalidPayload SignatureVerificationErrorCode = "invalid_payload"
)

// SignatureVerificationError is returned when webhook verification fails.
type SignatureVerificationError struct {
	Code    SignatureVerificationErrorCode
	Message string
}

func (e *SignatureVerificationError) Error() string {
	if e == nil {
		return ""
	}

	return e.Message
}

func (e *SignatureVerificationError) invoqSDKError() {}

func signatureError(code SignatureVerificationErrorCode, message string) *SignatureVerificationError {
	return &SignatureVerificationError{
		Code:    code,
		Message: message,
	}
}

func invalidResponseError(message string, cause error, payload any) *Error {
	return &Error{
		Message: message,
		Err:     cause,
		Payload: payload,
	}
}

func wrapResponseReadError(err error) *Error {
	return &Error{
		Message: "Failed to read invoq API response.",
		Err:     err,
	}
}

func wrapConnectError(err error) *Error {
	return &Error{
		Message: "Failed to connect to invoq API.",
		Err:     err,
	}
}

func wrapTimeoutError(err error) *Error {
	return &Error{
		Message: "invoq API request timed out.",
		Err:     err,
	}
}

func wrapParseResponseError(err error) *Error {
	return &Error{
		Message: "Failed to parse invoq API response.",
		Err:     err,
	}
}

func newAPIError(status int, payload any) *APIError {
	errorPayload, _ := payload.(map[string]any)

	code, _ := errorPayload["code"].(string)
	// An empty message the API really sent is kept; only an absent or wrong-typed
	// one falls back.
	message, ok := errorPayload["message"].(string)
	if !ok {
		message = "invoq API request failed."
	}

	apiError := &APIError{
		Message: message,
		Status:  status,
		Code:    code,
		Fields:  parseAPIErrorFields(errorPayload["fields"]),
		Meta:    parseAPIErrorMeta(errorPayload["meta"]),
		Payload: payload,
	}

	return apiError
}

func parseAPIErrorFields(value any) []APIErrorField {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	fields := make([]APIErrorField, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}

		// A location this version does not know is passed through, not dropped:
		// the caller is already on an error path and needs the code and message.
		// Only a structurally invalid entry is discarded.
		field, fieldOK := object["field"].(string)
		location, locationOK := object["location"].(string)
		code, codeOK := object["code"].(string)
		message, messageOK := object["message"].(string)
		if !fieldOK || !locationOK || !codeOK || !messageOK {
			continue
		}

		fields = append(fields, APIErrorField{
			Field:    field,
			Location: APIErrorLocation(location),
			Code:     code,
			Message:  message,
		})
	}

	// Empty but non-nil: nil is reserved for an absent or non-array `fields`.
	return fields
}

func parseAPIErrorMeta(value any) map[string]any {
	meta, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	return meta
}

func unexpectedMarshalError(err error) *Error {
	return &Error{
		Message: "Failed to parse invoq API response.",
		Err:     err,
	}
}
