package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	sdk "github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"

	"github.com/chpock/gen-commit-msg/internal/logging"
)

type thinkingSelection struct {
	messageID  string
	providerID string
	modelID    string
}

func (c *Client) prepareThinkingLevel(ctx context.Context, sessionID, level string) (thinkingSelection, error) {
	slog.Info("preparing OpenCode thinking level", "session_id", sessionID, "agent", c.agent, "thinking_level", level)
	params := sdk.SessionPromptParams{
		Directory: sdk.F(c.repoDir),
		Agent:     sdk.F(c.agent),
		NoReply:   sdk.F(true),
		Parts:     sdk.F([]sdk.SessionPromptParamsPartUnion{}),
	}
	if slog.Default().Enabled(ctx, logging.LevelTrace) {
		paramsJSON, _ := json.Marshal(params)
		slog.LogAttrs(ctx, logging.LevelTrace, "opencode request: thinking preparation",
			slog.String("session_id", sessionID), slog.String("params", string(paramsJSON)), slog.String("variant", level))
	}
	response, err := c.session.Prompt(ctx, sessionID, params, option.WithJSONSet("variant", level))
	if err != nil {
		return thinkingSelection{}, fmt.Errorf("prepare OpenCode thinking level: %w", err)
	}
	var prepared struct {
		Info struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionID"`
			Role      string `json:"role"`
			Model     struct {
				ProviderID string `json:"providerID"`
				ModelID    string `json:"modelID"`
				Variant    string `json:"variant"`
			} `json:"model"`
		} `json:"info"`
		Parts []json.RawMessage `json:"parts"`
	}
	if response == nil {
		return thinkingSelection{}, fmt.Errorf("OpenCode returned no thinking-level preparation")
	}
	if err := json.Unmarshal([]byte(response.JSON.RawJSON()), &prepared); err != nil {
		return thinkingSelection{}, fmt.Errorf("decode OpenCode thinking-level preparation: %w", err)
	}
	info := prepared.Info
	if info.Role != "user" || info.ID == "" || info.SessionID != sessionID || info.Model.ProviderID == "" || info.Model.ModelID == "" || info.Model.Variant != level {
		return thinkingSelection{}, fmt.Errorf("OpenCode did not return the requested thinking-level preparation with valid user/model metadata")
	}
	if prepared.Parts == nil || len(prepared.Parts) != 0 {
		return thinkingSelection{}, fmt.Errorf("OpenCode did not return an empty parts array during thinking-level preparation; cannot safely reuse the prepared message")
	}
	selection := thinkingSelection{messageID: info.ID, providerID: info.Model.ProviderID, modelID: info.Model.ModelID}
	if err := c.validateThinkingVariant(ctx, selection, level); err != nil {
		return thinkingSelection{}, err
	}
	slog.Info("OpenCode thinking level configured", "session_id", sessionID, "message_id", selection.messageID,
		"provider", selection.providerID, "model", selection.modelID, "thinking_level", level)
	return selection, nil
}

func (c *Client) validateThinkingVariant(ctx context.Context, selection thinkingSelection, level string) error {
	var catalog struct {
		Providers []struct {
			ID     string `json:"id"`
			Models map[string]struct {
				Variants map[string]json.RawMessage `json:"variants"`
			} `json:"models"`
		} `json:"providers"`
	}
	slog.Debug("reading OpenCode model variants", "provider", selection.providerID, "model", selection.modelID, "dir", c.repoDir)
	if err := c.api.Get(ctx, "config/providers", sdk.AppProvidersParams{Directory: sdk.F(c.repoDir)}, &catalog); err != nil {
		return fmt.Errorf("read OpenCode model variants: %w", err)
	}
	for _, provider := range catalog.Providers {
		if provider.ID != selection.providerID {
			continue
		}
		model, ok := provider.Models[selection.modelID]
		if !ok {
			break
		}
		available := make([]string, 0, len(model.Variants))
		for name := range model.Variants {
			available = append(available, name)
		}
		slices.Sort(available)
		slog.Debug("OpenCode model variants resolved", "provider", selection.providerID, "model", selection.modelID, "variants", available)
		variant, ok := model.Variants[level]
		if !ok || strings.TrimSpace(string(variant)) == "null" {
			return fmt.Errorf("OpenCode model %s/%s does not support thinking-level %q (available OpenCode variants: %q); choose a matching Pi level or omit --thinking-level",
				selection.providerID, selection.modelID, level, available)
		}
		var options map[string]json.RawMessage
		if err := json.Unmarshal(variant, &options); err != nil {
			return fmt.Errorf("invalid OpenCode variant %q for model %s/%s: %w", level, selection.providerID, selection.modelID, err)
		}
		return nil
	}
	return fmt.Errorf("OpenCode selected model %s/%s is absent from its provider catalog; cannot validate thinking-level %q", selection.providerID, selection.modelID, level)
}
