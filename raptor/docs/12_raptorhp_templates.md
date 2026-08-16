# Raptor Documentation Manual: RaptorHP Embedded Templates & Web Server

## 1. Embedded Template Syntax

RaptorHP allows embedding dynamic Raptor code blocks inside HTML or markdown documents using PHP-style tags:

| Tag Syntax | Purpose | Example |
| :--- | :--- | :--- |
| `<?raptor ... ?>` | Code execution block | `<?raptor my $name = "Quantum"; ?>` |
| `<?rp ... ?>` | Shorthand code execution | `<?rp for @items -> $i { ?>...<?rp } ?>` |
| `<?php ... ?>` | PHP compatibility tag | `<?php my $total = 100; ?>` |
| `<?= $expr ?>` | Output expression (echo shorthand) | `<h1>Welcome, <?= $user ?>!</h1>` |
| `<? ... ?>` | Short open tag | `<? say "Inline script"; ?>` |

## 2. Example Template (`index.phtml`)

```html
<!DOCTYPE html>
<html>
<head>
    <title>Dynamic RaptorHP Page</title>
</head>
<body>
    <?raptor
    my $title = "Real-Time Telemetry Dashboard";
    my @servers = [
        { "name" => "US-East", "status" => "Online", "ping" => 14 },
        { "name" => "EU-Central", "status" => "Online", "ping" => 22 },
        { "name" => "AP-South", "status" => "Maintenance", "ping" => 88 }
    ];
    ?>

    <h1><?= $title ?></h1>

    <table border="1" cellpadding="8">
        <tr>
            <th>Server Node</th>
            <th>Operational Status</th>
            <th>Latency</th>
        </tr>
        <?rp for @servers -> $srv { ?>
        <tr>
            <td><?= $srv{"name"} ?></td>
            <td><?= $srv{"status"} ?></td>
            <td><?= $srv{"ping"} ?> ms</td>
        </tr>
        <?rp } ?>
    </table>
</body>
</html>
```

## 3. CLI Template Execution

Render templates directly to standard output:

```powershell
# 1. Render file template
raptorhp index.phtml > output.html

# 2. Evaluate inline template string
raptorhp -r '<h1><?= "Hello " ~ "from RaptorHP!" ?></h1>'
```

## 4. Built-in Development Web Server

Start a local HTTP server that parses and executes `.phtml`, `.rhtml`, `.rp`, `.raptor`, and `.html` templates dynamically:

```powershell
# Same as php -S localhost:8000
raptor -S localhost:8000
raptorhp -S localhost:8000

# Document root (php -S ... -t public)
raptor -S localhost:8000 -t public
raptorhp -S localhost:8000 -t public

# Optional front-controller
raptor -S localhost:8000 router.phtml
```

### Predefined Superglobals in Web Server Mode

| Superglobal | Variable | Description |
| :--- | :--- | :--- |
| `%*GET` / `%_GET` | `%_GET{"key"}` | URL query parameters |
| `%*POST` / `%_POST` | `%_POST{"field"}` | Form POST payload fields |
| `%*SERVER` / `%_SERVER` | `%_SERVER{"REQUEST_METHOD"}` | HTTP method, URI, remote address, headers |
| `%*ENV` / `%ENV` | `%*ENV{"PATH"}` | Host system environment variables |

Example using query parameters:
```html
<p>Searching for query: <?= %_GET{"q"} // "None specified" ?></p>
<p>Client IP: <?= %_SERVER{"REMOTE_ADDR"} ?></p>
```
