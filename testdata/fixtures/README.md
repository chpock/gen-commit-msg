# OpenCode API Response Fixtures

This directory contains JSON fixture files representing typical responses from the
OpenCode API. These fixtures are used by integration tests to mock the OpenCode
SDK without requiring a real OpenCode server.

## How to add new fixtures

1. Capture a real response from the OpenCode API (e.g. from logs or by inspecting
   the SDK response in a debugger).
2. Save the JSON response as a new `.json` file in this directory.
3. Name the file descriptively (e.g. `prompt_success_many_subjects.json`).
4. If adding a prompt response, ensure it includes the `structured` or
   `structured_output` field within `info`.
5. Register the fixture in the appropriate scenario map in
   `internal/opencode/fixtures.go`.

## Fixture naming convention

- `session_create_*.json` — Responses for `Session.New`
- `prompt_*.json` — Responses for `Session.Prompt`
- `session_delete_*.json` — Responses for `Session.Delete`

## Response format

The JSON in each file should match the raw response returned by the OpenCode API.
The SDK will deserialize it into the appropriate Go types.

### Session.Create response (returns `opencode.Session`)
```json
{
  "id": "session_id",
  "directory": "/path/to/repo",
  "projectID": "project_id",
  "time": {"created": 1715616000, "updated": 1715616001},
  "title": "agent-name",
  "version": "1.0"
}
```

### Session.Prompt response (returns `opencode.SessionPromptResponse`)
```json
{
  "info": {
    "id": "msg_id",
    "sessionID": "session_id",
    "role": "assistant",
    "mode": "chat",
    "modelID": "model-id",
    "providerID": "provider-id",
    "parentID": "parent_id",
    "path": {"cwd": "/path", "root": "/path"},
    "time": {"created": 1715616000, "completed": 1715616005},
    "tokens": {"input": 100, "output": 50, "reasoning": 0,
               "cache": {"read": 0, "write": 0}},
    "cost": 0.01,
    "system": [],
    "structured": "{\"subjects\":[\"feat: example\"],\"body\":\"Example body.\"}"
  },
  "parts": []
}
```

### Session.Delete response (returns `bool`)
Any valid JSON that maps to a boolean, e.g. `true`.
