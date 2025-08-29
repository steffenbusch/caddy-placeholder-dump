# caddy-placeholder-dump

The **caddy-placeholder-dump** plugin for [Caddy](https://caddyserver.com) allows you to log or write resolved placeholders to a file. But this plugin is not just useful for debugging or monitoring placeholder values in your Caddy configuration, it can be used for other purposes as well.

## Features

- **Write to File**: Dump resolved placeholder values to a specified file.
- **Log to Logger**: Log resolved placeholder values to a logger with a customizable suffix.
- **Flexible Configuration**: Supports both file-based and logger-based outputs, or both simultaneously.

## Configuration

The `placeholder_dump` directive can be configured in the Caddyfile. Below is an example configuration, followed by detailed explanations of each configuration option.

> [!NOTE]
> The `placeholder_dump` directive has **no default order** in the Caddyfile.
> This means it  should be used within a `route` directive to explicitly define its order relative to other handlers.
> Alternatively, the `order` directive in the global options must be used to specify its order.

### Example Caddyfile Configuration

```caddyfile
:8080 {
    route {
        placeholder_dump {
            content "Resolved placeholder: {http.request.uri}"
            file "/path/to/output.log"
            logger_suffix "custom_logger"
        }

        respond "Placeholder values have been logged or written to a file."
    }
}
```

### Configuration Options

- **`content`**: (Required) The content to be logged or written to a file. This can include placeholders that will be resolved at runtime.
  - Example: `"Resolved placeholder: {http.request.uri}"`

- **`file`**: (Optional) The path to the file where the resolved content will be written. This option supports placeholder replacements, so you can use placeholders in the file path that will be resolved at runtime. If the file does not exist, it will be created.
  - Examples: `"/path/to/output.log"` or `"/path/to/{time.unix.now}_trace.log"`

- **`file_permissions`**: (Optional) The permissions (in octal string format, e.g. `"644"` or `"0600"`) to use when creating the file specified by `file`. Default is `"644"`. This controls the access rights for the created file.
  - Example: `"600"` (owner read/write only), `"644"` (owner read/write, others read)

- **`logger_suffix`**: (Optional) A suffix appended to the module's logger name (`http.handlers.placeholder_dump`). The resolved content will be logged to this logger.
  - Example: `"custom_logger"` (content will be logged at `http.handlers.placeholder_dump.custom_logger`)

> [!NOTE]
> At least one of `file` or `logger_suffix` must be configured. Both can be used simultaneously.

> [!IMPORTANT]
> **File Writes vs. Logger Usage**: For every write operation to the file, the file is opened and closed. This makes the file option unsuitable for heavy or frequent writes. It is better suited for rare or occasional writes. For scenarios requiring heavy logging, consider using the `logger_suffix` option with a dedicated log configuration for better performance and scalability.

## How It Works

1. **Content Resolution**: The `content` field can include multiple placeholders (e.g., `{http.auth.user.id}`, `{http.request.method}`), which are resolved at runtime using Caddy's replacer.

2. **Output Options**:

- If `file` is configured, the resolved content is written to the specified file.
- If `logger_suffix` is configured, the resolved content is logged to the logger with the specified suffix.
- If both are configured, the content is both logged and written to the file.

## Example Use Cases

### Simple Placeholder Value Debugging

Log placeholder values to a file for debugging purposes:

```caddyfile
placeholder_dump {
    content "Request URI: {http.request.uri}"
    file "/var/log/caddy/placeholders.log"
    file_permissions 600
}
```

### Dump Request Body

Write request body of API requests to a file when the query parameter "trace=true" is used:

```caddyfile
  route /api/* {
    @dump_body {
        query trace=true
    }

    placeholder_dump @dump_body {
      content "{time.now.unix}: {method} request fom {client_ip} with body: {http.request.body}"
      file "/var/log/caddy/api_trace_{time.now.unix}.log"
    }

    reverse_proxy localhost:9001
  }
```

> [!NOTE]
> Consider using the `log_append` directive to conditionally include the request body in your access log.

### Append JWT (JTI) to Blacklist on Logout

This example demonstrates how to use the `placeholder_dump` directive to append the JWT's `jti` (JWT ID) claim to a blacklist file upon logout.

```caddyfile
  route /protected/* {

    # see https://github.com/ggicci/caddy-jwt
    jwtauth {
      sign_key {file.jwt_sign_key_secret.txt}
      sign_alg HS256
      issuer_whitelist https://auth.example.com
      user_claims sub
      meta_claims "jti"
    }

    placeholder_dump /protected/logout* {
      content "{http.auth.user.jti}"
      file "/var/log/caddy/jwt_blacklist.txt"
    }

    reverse_proxy localhost:9001
  }
```

### Advanced Logging for JWT Monitoring

This example demonstrates how to log JWT-related placeholder values, such as the `jti` (JWT ID), to a dedicated logger (`http.handlers.placeholder_dump.visited_jti`).

For this logger, the `log` directive is used in the global options block to specify that the messages for this logger are written to a particular file.
The file settings include configurable permissions (`mode`), rollover settings (`roll_size` and `roll_keep_for`), and the use of the `transform` encoder to only include the `content` property in the log output.

```caddyfile
{
  log jti_visitor {
    output file log/jti-visitor.log {
      mode 600
      roll_size 1M
      roll_keep_for 7d
    }
    # see https://github.com/caddyserver/transform-encoder
    format transform "{content}"
    include http.handlers.placeholder_dump.visited_jti
   }
}

app.example.com {
  # For the {extra.time.now.custom} placeholder
  # see https://github.com/steffenbusch/caddy-extra-placeholders
  extra_placeholders

  error /favicon.ico 404

  route {
    # see https://github.com/ggicci/caddy-jwt
    jwtauth {
      sign_key {file.jwt_sign_key_secret.txt}
      sign_alg HS256
      issuer_whitelist https://auth.example.com
      audience_whitelist "app.example.com"
      user_claims sub
      meta_claims "jti" "ip" "name"
    }

    placeholder_dump {
      content "{extra.time.now.custom} User {http.auth.user.id} with JTI {http.auth.user.jti} was here."
      logger_suffix visited_jti
    }

    reverse_proxy localhost:9001
  }
}
```

## Building

To build Caddy with this module, use `xcaddy`:

```bash
xcaddy build --with github.com/steffenbusch/caddy-placeholder-dump
```

## Security Considerations

- **File Permissions**: The file specified in the `file` option is created with permissions set by the `file_permissions` option (default: `644`, readable by everyone, writable by the owner). You can set stricter permissions (e.g., `600`) if the file contains sensitive information.
- **Log Monitoring**: If using the `logger_suffix` option, ensure logs are monitored and stored securely to prevent sensitive information leakage.

## License

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.

## Acknowledgements

- [Caddy](https://caddyserver.com) for providing a powerful and extensible web server.
