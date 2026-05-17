package internal

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
)

type SimulatorDevice struct {
	Name    string
	Runtime string
	UDID    string
}

type SimctlRunner func(args ...string) ([]byte, error)

func RunSimctl(args ...string) ([]byte, error) {
	cmd := exec.Command("xcrun", append([]string{"simctl"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running xcrun simctl %v: %w", args, err)
	}
	return output, nil
}

func ListUnavailableSimulators(run SimctlRunner) ([]SimulatorDevice, error) {
	type simctlDevice struct {
		Name        string `json:"name"`
		UDID        string `json:"udid"`
		IsAvailable bool   `json:"isAvailable"`
	}

	type simctlDevices struct {
		Devices map[string][]simctlDevice `json:"devices"`
	}

	output, err := run("list", "devices", "--json")
	if err != nil {
		return nil, fmt.Errorf("listing simulator devices: %w", err)
	}

	var payload simctlDevices
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parsing simctl devices json: %w", err)
	}

	devices := make([]SimulatorDevice, 0)
	for runtime, runtimeDevices := range payload.Devices {
		for _, device := range runtimeDevices {
			if device.IsAvailable {
				continue
			}
			devices = append(devices, SimulatorDevice{
				Name:    device.Name,
				Runtime: runtime,
				UDID:    device.UDID,
			})
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Runtime != devices[j].Runtime {
			return devices[i].Runtime < devices[j].Runtime
		}
		if devices[i].Name != devices[j].Name {
			return devices[i].Name < devices[j].Name
		}
		return devices[i].UDID < devices[j].UDID
	})

	return devices, nil
}

func DeleteUnavailableSimulators(run SimctlRunner) error {
	_, err := run("delete", "unavailable")
	if err != nil {
		return fmt.Errorf("deleting unavailable simulators: %w", err)
	}
	return nil
}
