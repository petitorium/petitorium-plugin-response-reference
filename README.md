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

| Parameter   | Required | Description                                                                 |
|-------------|----------|-----------------------------------------------------------------------------|
| `request`   | Yes      | Name of the source request, or its full slash-separated collection path     |
| `attribute` | Yes      | One of: `body`, `header`, `status`                                          |
| `jsonPath`  | No*      | JSONPath expression when `attribute=body`                                   |
| `headerName`| No*      | Header name when `attribute=header`                                         |

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

## Referencing requests in nested collections

Collections can be nested. If the source request has a **unique name** across the workspace, you can reference it by name alone:

```
{{response-reference:ref request="index" attribute="body" jsonPath="object.0.id"}}
```

If multiple requests share the same name, use the **full slash-separated collection path** to disambiguate. For a request located at `op-salary/beneficiaries/index`:

```json
{
  "idBeneficiary": "{{response-reference:ref request="op-salary/beneficiaries/index" attribute="body" jsonPath="object.2.idBeneficiary"}}"
}
```

When a name is ambiguous, the tag editor dropdown will automatically use the full path as the value. You can also type the path manually. If you use a path that does not exist, the error message lists all available request paths in the workspace.

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
