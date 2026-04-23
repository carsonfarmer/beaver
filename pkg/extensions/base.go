package extensions

import (
	"context"
	"fmt"

	acp "github.com/ironpark/go-acp"
)

// BasePrompt returns a provider that contributes the given base prompt,
// appending the working directory when one is set.
func BasePrompt(text string) Provider {
	return func(_ context.Context, cwd string, _ acp.SessionID) (string, error) {
		prompt := text
		if cwd != "" {
			prompt += fmt.Sprintf(" The working directory is %s. Always use absolute paths when reading or writing files.", cwd)
		}
		return prompt, nil
	}
}
