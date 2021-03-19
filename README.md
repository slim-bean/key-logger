A key logger which outputs to a Loki friendly format.

Currently only built for Windows.

```
go build -o cmd/key-logger/key-logger.exe ./cmd/key-logger/main.go
```

Use promtail to send to Loki

```
key-logger.exe | promtail-windows-amd64.exe -config.file=promtail-config.yaml --stdin
```

Example promtail config:

```

clients:
  - url: http://localhost:3100/loki/api/v1/push  

scrape_configs:
  - job_name: system
    static_configs:
      - labels:
          job: keylogger  # We are using the --stdin flag so logs come from stdin, this flag uses the first defined scrape config which is this config, set a label job=keylogger
          host: winbookpro
    pipeline_stages:
      - replace:
          expression: "(?i).*(password).*" # If you want to ignore passswords add replace expressions 
      - replace:
          expression: "(?i).*(pass.*d).*" # Add as many as passwords you regularly use, if you don't want to type out the entire password, put a .* in the middle of it
      - replace:
          expression: "(?i).*(pass.*1).*" # NOTE because of how [shift] works, special chars in passwords will only log as numbers, this would match password1 OR password!
```

