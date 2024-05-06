# tplagent

## Generate and refresh files dynamically using Go templates

`tplagent` can be invoked as a daemon process on your VMs to dynamically generate config files / any kind of content
and refresh them periodically based on a JSON config file. This project is inspired by telegraf agent and the consul
template projects, it combines certain aspects of both projects to provide a more generic interface for generating and
refreshing config files.

## Installation instructions

- Download the latest version of `tplagent` from the GitHub releases section.
- Generate a starter configuration by using the `tplagent genconf` command

```shell
    tplagnet genconf -n <number_of_template_blocks> -indent <json_indent> > /path/to/config.json
```

- Start the agent using the `tplagent start` command

```shell
tplagent start -config /path/to/config.json

```

## Configuration explained

```json
{
  "agent": {
    // set agent log level
    "log_level": "INFO",
    // set agent log format
    "log_fmt": "text",
    // set max consecutive failures for command execution and 
    // template execution
    "max_consecutive_failures": 10
  },
  "templates": {
    "nginx-conf": {
      "source": "/etc/nginx/nginx.conf.tmpl",
      // path to read the template file from
      "destination": "/etc/nginx/nginx.conf",
      // path to render the config file to
      "html": false,
      // enable this flag if template is an HTML template
      // this will parse the template by escaping 
      // JS code injections
      "static_data": {
        "MaxConnections": 100
      },
      // static data is a key value pair for
      // data which will not change
      "refresh_interval": "15s",
      // how frequently to refresh the 
      // destination file
      "missing_key": "error",
      // used to specify the missing key in 
      // the template data
      "exec": {
        "cmd": "service",
        "cmd_args": [
          "nginx",
          "reload"
        ],
        // run a command
        // after rendering the
        // template
        "cmd_timeout": "30s",
        // command execution timeout
        "env": {
          "CERT_FILE_PATH": "/etc/certfiles/nginx.cert"
        }
        // extra env for the command
      }
    },
    "credentials-json": {
      "actions": [
        {
          "name": "httpjson",
          "config": {
            "base_url": "http://cloud-provider.com",
            "timeout": "10s"
          }
        }
      ],
      "raw": "{{with httpjson_GET_Map \"/v1/creds\"}}\n{\"secret_key\":\"{{.SecretKey}}\"}\n{{end}}",
      "destination": "/etc/cloud-provider/creds.json",
      "refresh_interval": "1h",
      "missing_key": "error"
    }
  }
}
```

## On How to use Go templates properly please refer to

https://pkg.go.dev/text/template