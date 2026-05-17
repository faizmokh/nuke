package internal

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestListUnavailableSimulators(t *testing.T) {
	runner := func(args ...string) ([]byte, error) {
		want := []string{"list", "devices", "--json"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("args = %#v, want %#v", args, want)
		}
		return []byte(`{
			"devices": {
				"com.apple.CoreSimulator.SimRuntime.iOS-17-5": [
					{"name":"iPhone 15","udid":"AAA","isAvailable":true},
					{"name":"iPhone 8","udid":"BBB","isAvailable":false}
				],
				"com.apple.CoreSimulator.SimRuntime.iOS-16-4": [
					{"name":"iPad Pro","udid":"CCC","isAvailable":false}
				]
			}
		}`), nil
	}

	devices, err := ListUnavailableSimulators(runner)
	if err != nil {
		t.Fatalf("ListUnavailableSimulators() error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(devices))
	}
	if devices[0].Name != "iPad Pro" || devices[0].Runtime != "com.apple.CoreSimulator.SimRuntime.iOS-16-4" {
		t.Fatalf("devices[0] = %#v, want iPad Pro on iOS-16-4", devices[0])
	}
	if devices[1].Name != "iPhone 8" || devices[1].Runtime != "com.apple.CoreSimulator.SimRuntime.iOS-17-5" {
		t.Fatalf("devices[1] = %#v, want iPhone 8 on iOS-17-5", devices[1])
	}
}

func TestDeleteUnavailableSimulators(t *testing.T) {
	called := false
	runner := func(args ...string) ([]byte, error) {
		called = true
		want := []string{"delete", "unavailable"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("args = %#v, want %#v", args, want)
		}
		return nil, nil
	}

	if err := DeleteUnavailableSimulators(runner); err != nil {
		t.Fatalf("DeleteUnavailableSimulators() error: %v", err)
	}
	if !called {
		t.Fatal("expected runner to be called")
	}
}

func TestListUnavailableSimulatorsInvalidJSON(t *testing.T) {
	runner := func(args ...string) ([]byte, error) {
		return []byte("not-json"), nil
	}

	_, err := ListUnavailableSimulators(runner)
	if err == nil {
		t.Fatal("ListUnavailableSimulators() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parsing simctl devices json") {
		t.Fatalf("error = %q, want parse context", err)
	}
}

func TestListUnavailableSimulatorsRunnerError(t *testing.T) {
	runner := func(args ...string) ([]byte, error) {
		return nil, errors.New("simctl unavailable")
	}

	_, err := ListUnavailableSimulators(runner)
	if err == nil {
		t.Fatal("ListUnavailableSimulators() error = nil, want runner error")
	}
	if !strings.Contains(err.Error(), "listing simulator devices") {
		t.Fatalf("error = %q, want list context", err)
	}
}

func TestDeleteUnavailableSimulatorsRunnerError(t *testing.T) {
	runner := func(args ...string) ([]byte, error) {
		return nil, errors.New("delete failed")
	}

	err := DeleteUnavailableSimulators(runner)
	if err == nil {
		t.Fatal("DeleteUnavailableSimulators() error = nil, want runner error")
	}
	if !strings.Contains(err.Error(), "deleting unavailable simulators") {
		t.Fatalf("error = %q, want delete context", err)
	}
}
