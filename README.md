# Petitorium Plugin: Response Reference

Extract values from other requests' response history and inject them as template variables.

## Usage

In any request field (URL, body, or header), insert a tag like:

```json
{
  "token": "{{response-reference:ref request="Login" attribute="body" jsonPath="data.token"}}"
}
```

Before the request is sent, the plugin will look up the most recent response from the **Login** request and extract the value at JSONPath `data.token`.

## Tag Attributes

| Parameter   | Required | Description                                               |
|-------------|----------|-----------------------------------------------------------|
| `request`   | Yes      | Name of the source request (dropdown in tag editor)       |
| `attribute` | Yes      | One of: `body`, `header`, `status`                        |
| `jsonPath`  | No*      | JSONPath expression when `attribute=body`                 |
| `headerName`| No*      | Header name when `attribute=header`                       |

\* Required when the corresponding `attribute` is selected.

## Examples

### Extract a JSON field from the response body
```
{{response-reference:ref request="Get Token" attribute="body" jsonPath="object.access_token"}}
```

### Extract a response header
```
{{response-reference:ref request="Get Token" attribute="header" headerName="X-Request-Id"}}
```

### Extract the status code
```
{{response-reference:ref request="Get Token" attribute="status"}}
```

## How it works

1. The plugin registers for the `PreVariableSubstitution` hook.
2. It scans the outgoing request URL, body, and headers for `{{response-reference:ref ...}}` tags.
3. It reads `~/.config/petitorium/workspaces/<workspace>/collections.yaml`.
4. It finds the referenced request and takes its most recent response from `response_history`.
5. It extracts the requested value and replaces the tag inline.

## Building

```bash
make build
```

Cross-compile for all platforms:
```bash
make all
```

## Testing

```bash
make test
```
