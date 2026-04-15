package tools

import (
	"encoding/json"
	"fmt"
)

// UnmarshalToolInputJSON decodes the tool's JSON input string into dest (a pointer or map).
// It wraps json.Unmarshal errors with a stable "invalid json input" prefix for model-facing errors.
func UnmarshalToolInputJSON(input string, dest any) error {
	if err := json.Unmarshal([]byte(input), dest); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}
	return nil
}
