## httpjson

### Contains actions to make quick REST API calls in your template to refresh data

Example:

```gotemplate
{{with http_json_GET_Map "/v1/secrets/user123" }}
{
  "secret_token": "{{.Token}}"
}
{{end}}
```

Output:

Assuming the data returned by the endpoint is `{"Token":"ffd456"}`

```json5
{
  "secret_token": "ffd456"
}
```

Example:

```gotemplate
{{with http_json_GET_Slice "/v1/env_vars?app_name=foo" }}
{{range $index,$env_var := .}}
{{.env_var}}
{{end}}
{{end}}
```

Output:
Assuming the data returned by the endpoint is `["export NAME=foo","export PORT=9090"]`

```shell
export NAME=foo
export PORT=9090
```