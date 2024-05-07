# tplagent

<!-- TOC -->
* [tplagent](#tplagent)
  * [Generate and refresh config/any type of files dynamically using Go templates](#generate-and-refresh-configany-type-of-files-dynamically-using-go-templates)
  * [Installation instructions](#installation-instructions)
  * [Supported Platforms](#supported-platforms)
  * [Directory Permissions](#directory-permissions)
  * [Reloading the agent](#reloading-the-agent)
  * [Configuration explained](#configuration-explained)
  * [Actions](#actions)
    * [How to invoke a certain action](#how-to-invoke-a-certain-action)
    * [Contributing new actions](#contributing-new-actions)
  * [On How to use Go templates properly please refer to](#on-how-to-use-go-templates-properly-please-refer-to)
<!-- TOC -->
## Generate and refresh config/any type of files dynamically using Go templates

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
**NOTE** 
It is recommended to run the agent as a daemon process by creating and configuring a valid systemd unit file. This way the agent can be restarted inspite of system reboots.
It is much easier to reload the agent.

## Supported Platforms

Windows is not supported. Only Linux and macOS are supported. PRs are welcome to add support for windows

## Directory Permissions

It is recommended that the `tplagent` process is started under a dedicated user meant for `tplagent`. All directories
are files created by `tplagent` use the `766` permissions. It is also recommended to manage directory permissions as
part of setting up the agent to avoid permission errors.

## Reloading the agent

Agent can be reloaded by sending a SIGHUP signal to the agent process, if the agent process starts up successfully the
PID is stored inside
`/tmp/tplagent/agent.pid`. Agent reloading can be useful to read new config

## Configuration explained

```json5
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
      // specifies how frequently to
      // render the template file
      // min is 1s
      // for valid duration units 
      // please refer to
      // https://pkg.go.dev/maze.io/x/duration#ParseDuration
      "missing_key": "error",
      // used to specify the missing key behaviour in 
      // the template data
      // for other valid values
      // please refer to https://pkg.go.dev/text/template#Template.Option
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
          "DATA_DIR": "/var/lib/data"
        }
        // extra env vars for the command
      }
    },
    "credentials-json": {
      // actions are functions you want 
      // included in your template
      "actions": [
        {
          "name": "httpjson",
          "config": {
            "base_url": "http://cloud-provider.com",
            "timeout": "10s"
          }
        }
      ],
      // if source is not specified, we use the raw option to
      // specify an inline template
      "raw": "<<with httpjson_GET_Map \"/v1/creds\">>\n{\"secret_key\":\"<<.SecretKey>>\"}\n<<end>>",
      "render_once": true,
      // render_once renders it only once
      // and does not refresh preiodically
      "destination": "/etc/cloud-provider/creds.json",
      "missing_key": "error",
      "template_delimiters": [
        "<<",
        ">>"
      ],
      // the default delimiters are {{ and }}
      // this option lets you change the delimiters
    }
  }
}
```

## Actions

What makes `tplagent` dynamic and extensible are the actions. Actions are just plain functions you can call in your
templates to perform certain actions. Their main utility is to fetch data from different sources. New actions can be
added to `tplagent`, which will be covered later. An action is just a collection of functions which can be used in your
template file. You can include multiple actions in a single template, to invoke a certain actions in the template, it's
corresponding action must be included in the `"actions"` section of the template config.

### How to invoke a certain action

- An action can be invoked by prefixing its name followed by the function you want to invoke, for example the `httpjson`
  actions contain the functions `GET_Map` and `GET_Slice`, these functions get and unmarshall the data as a Go hash map
  and a Go
  slice respectively. To invoke them in my template I would use it in the following way

```gotemplate
{{with httpjson_GET_Map "/v1/someapi"}}
{
"user_id": "{{.UserID}}",
"secret_key": "{{.SecretKey}}"
}
{{end}}
```

- The above example assumes that the JSON returned by the API looks like this

```json
{
  "UserID": "foo",
  "SecretKey": "foo-secret"
}
```

### Contributing new actions

Prerequisites:

- Understanding of the Golang language and how interfaces work in Golang.
- Understanding of the `init` function in Golang and the `_` import.

- There is just one action included with the agent currently, any new action requires the agent to be rebuilt as all
  actions get packaged in the same binary.
- All actions follow the `tplactions.Interface`. Each action must be created in its own package and the config for each
  action, will be sent as raw json, and it is upto the action implementation to deserialize and store the config
- Please add your action inside the `internal/tplactions/<your_action_name>/...`
- After an action has been written and tested, it must be imported using the **underscore** import
  inside `internal/agent/register_template_actions.go`.
- Raise a PR to include the action in the next release cycle.

**NOTE**
If your action is very specific to your organisation / business, please clone the repository and create a custom build.
We want to include actions which can be used by most people.

## On How to use Go templates properly please refer to

https://pkg.go.dev/text/template
