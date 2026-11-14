package pi

import (
	"context"
	"slices"
	"testing"
)

func TestGeneratorThinkingLevel(t *testing.T) {
	for _, level := range []string{"", "off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("level/"+level, func(t *testing.T) {
			g, dir := newFixture(t, "correct-two")
			request := requestFixture()
			request.ThinkingLevel = level
			result, err := g.Generate(context.Background(), request, nil)
			if err != nil || len(result.Messages) != 1 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			events := fixtureEvents(t, dir)
			assertReaped(t, events)
			rpc, prompts := 0, 0
			for _, event := range events {
				if event.Kind == "rpc" {
					rpc++
					index := slices.Index(event.Args, "--thinking")
					if level == "" {
						if index >= 0 {
							t.Fatalf("unset level overrides Pi settings: %q", event.Args)
						}
					} else if index < 0 || index+1 >= len(event.Args) || event.Args[index+1] != level {
						t.Fatalf("thinking level %q not passed to Pi: %q", level, event.Args)
					}
					if slices.Contains(event.Args, "--model") || slices.Contains(event.Args, "--provider") {
						t.Fatalf("thinking override changes native model selection: %q", event.Args)
					}
				}
				if event.Kind == "command" && event.Command["type"] == "prompt" {
					prompts++
				}
			}
			if rpc != 1 || prompts != 2 {
				t.Fatalf("thinking override changed correction lifecycle: rpc=%d prompts=%d", rpc, prompts)
			}
		})
	}
}
