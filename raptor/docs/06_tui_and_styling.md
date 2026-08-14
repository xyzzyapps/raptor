# Raptor Charmbracelet TUI & Terminal Styling

Raptor includes a native **Charmbracelet (Lip Gloss / Bubble Tea / Glamour)** compatible terminal styling and TUI engine.

## 1. Lip Gloss ANSI Styling (`tui_style`)

Style text with 24-bit TrueColor, background colors, borders, margins, and typography:

```perl
# Styled text banner
my $banner = tui_style("RAPTOR RUNTIME", {
    "fg" => "#38bdf8",     # Sky blue
    "bg" => "#0f172a",     # Slate dark background
    "bold" => True,
    "border" => "rounded",
    "border_fg" => "#f43f5e",
    "padding" => [1, 2],
    "align" => "center"
});

say $banner;
```

## 2. Framed Boxes (`tui_box`)

```perl
my $box = tui_box("Server Status: Online\nActive Port: 8080\nUptime: 99.99%", {
    "title" => "System Monitor",
    "border" => "rounded",
    "border_fg" => "#4ade80" # Green
});

say $box;
```

## 3. Data Tables (`tui_table`)

```perl
my @headers = ["ID", "Service", "Status", "Latency"];
my @rows = [
    ["1", "Auth Core", "ONLINE", "1.2ms"],
    ["2", "PostgreSQL", "HEALTHY", "0.8ms"],
    ["3", "Redis Cache", "SYNCED", "0.3ms"]
];

say tui_table(@headers, @rows);
```

## 4. Progress Bars (`tui_progress`)

```perl
# Render 75% progress bar
say tui_progress(0.75, { "width" => 40, "fg" => "#facc15" });
```

## 5. Terminal Markdown Rendering (`tui_markdown`)

```perl
my $doc = "# Welcome to Raptor\n\n- Fast execution\n- Native sockets\n- **No OOP overhead**";
say tui_markdown($doc);
```

## 6. Bubble Tea Interactive App (`tui_app_run`)

Raptor supports Elm-style Model-Update-View event loops for terminal applications:

```perl
tui_app_run({
    "init" => sub () {
        return { "count" => 0, "status" => "Ready" };
    },
    "update" => sub ($model, $msg) {
        $model{"count"} = $model{"count"} + 1;
        return $model;
    },
    "view" => sub ($model) {
        return tui_box("Count: " ~ $model{"count"}, { "title" => "Counter App" });
    }
});
```
