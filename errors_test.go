package gpflag

import (
	"errors"
	"testing"
)

func TestNotExistError(t *testing.T) {
	err := &NotExistError{
		name:                "foo",
		specifiedShorthands: "bar",
	}

	if err.GetSpecifiedName() != "foo" {
		t.Errorf("Expected GetSpecifiedName to return %q, got %q", "foo", err.GetSpecifiedName())
	}
	if err.GetSpecifiedShortnames() != "bar" {
		t.Errorf("Expected GetSpecifiedShortnames to return %q, got %q", "bar", err.GetSpecifiedShortnames())
	}
}

func TestValueRequiredError(t *testing.T) {
	err := &ValueRequiredError{
		flag:                &Flag{},
		specifiedName:       "foo",
		specifiedShorthands: "bar",
	}

	if err.GetFlag() == nil {
		t.Error("Expected GetSpecifiedName to return its flag field, but got nil")
	}
	if err.GetSpecifiedName() != "foo" {
		t.Errorf("Expected GetSpecifiedName to return %q, got %q", "foo", err.GetSpecifiedName())
	}
	if err.GetSpecifiedShortnames() != "bar" {
		t.Errorf("Expected GetSpecifiedShortnames to return %q, got %q", "bar", err.GetSpecifiedShortnames())
	}
}

func TestInvalidValueError(t *testing.T) {
	expectedCause := errors.New("error")
	err := &InvalidValueError{
		flag:  &Flag{},
		value: "foo",
		cause: expectedCause,
	}

	if err.GetFlag() == nil {
		t.Error("Expected GetSpecifiedName to return its flag field, but got nil")
	}
	if err.GetValue() != "foo" {
		t.Errorf("Expected GetValue to return %q, got %q", "foo", err.GetValue())
	}
	if actual := err.Unwrap(); actual != expectedCause { //nolint:errorlint // Verify that Unwrap returns the exact cause, not a wrapper.
		t.Errorf("Expected Unwrap to return %q, got %q", expectedCause, actual)
	}
	if !errors.Is(err, expectedCause) {
		t.Errorf("Expected errors.Is to match %q, got %q", expectedCause, err)
	}
}

func TestInvalidSyntaxError(t *testing.T) {
	err := &InvalidSyntaxError{
		specifiedFlag: "--=",
	}

	if err.GetSpecifiedFlag() != "--=" {
		t.Errorf("Expected GetSpecifiedFlag to return %q, got %q", "--=", err.GetSpecifiedFlag())
	}
}
