package api

import (
	"github.com/api-go/cast"
	"errors"
)

// MockInputError identifies a rejected test input, rather than a broken fixture
// or failed execution. Callers can record this expected negative outcome.
type MockInputError struct{}

func (*MockInputError) Error() string { return "payload does not match declared mock test inputs" }

// callMock never opens credentials, signs, runs executable filters or connects
// to a provider. The response is a persisted fixture, not caller-supplied data.
func callMock(action Action, config map[string]any, opts *CallOptions, result *Result) (*Result, error) {
	result.Mode, result.Simulated = "mock", true
	fail := func(message string) (*Result, error) { result.Error = message; return result, errors.New(message) }
	if opts.Context != nil && opts.Context.Err() != nil {
		return fail(opts.Context.Err().Error())
	}
	if opts.Stream != nil {
		return fail("mock mode does not support streaming")
	}
	fixture, ok := config["mock"].(map[string]any)
	if !ok || !cast.Bool(fixture["enabled"]) {
		return fail("mock fixture is not enabled")
	}
	expected, ok := fixture["request"].(map[string]any)
	if !ok || len(expected) == 0 {
		return fail("mock requires declared test inputs")
	}
	var payload map[string]any
	if generic, ok := action.(*GenericAction); ok {
		payload = generic.payload
	} else {
		cast.Convert(&payload, action)
	}
	for key, value := range expected {
		actual, exists := payload[key]
		wantJSON, wantErr := cast.ToJSON(value)
		actualJSON, actualErr := cast.ToJSON(actual)
		if !exists || wantErr != nil || actualErr != nil || string(wantJSON) != string(actualJSON) {
			result.Error = (&MockInputError{}).Error()
			result.Code = "mock_input_rejected"
			return result, &MockInputError{}
		}
	}
	response, ok := fixture["response"].(map[string]any)
	if !ok {
		return fail("mock response is required")
	}
	result.StatusCode = cast.Int(response["statusCode"])
	if result.StatusCode < 100 || result.StatusCode > 599 {
		return fail("mock response requires a valid HTTP statusCode")
	}
	encoded, err := cast.ToJSON(response["data"])
	if err != nil {
		return fail("invalid mock response data")
	}
	result.Data, err = cast.FromJSON[any](string(encoded))
	if err != nil {
		return fail("invalid mock response data")
	}
	applyResponseRules(config, result)
	return result, nil
}
