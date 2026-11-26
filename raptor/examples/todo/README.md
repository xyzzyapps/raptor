# Raptor Full-Stack Todo Web Application

A complete, self-contained Todo Web Application built natively on the **Raptor** language runtime using `http_server_listen`, `sqlite_open` / `sqlite_exec` / `sqlite_query`, `json_encode` / `json_decode`, and **RaptorHP** server-rendered templates.

It ships with **two runnable full-stack modes** backed by the same SQLite schema:

1. **`app.rp` — Standalone REST JSON Microservice + Glassmorphism SPA**
   Single-file HTTP server serving both a modern dark/light responsive HTML5 + CSS3 + vanilla JS Single-Page App (`GET /`) and a RESTful JSON API (`/api/todos`).
2. **`index.phtml` — Server-Side Rendered RaptorHP Template App**
   Zero-JS-required PHP-style template (`<?raptor ... ?>` / `<?= ... ?>`) using `%_GET`, `%_POST`, and SQLite to list, create, edit, toggle, filter, search, and delete tasks purely server-side with HTML forms.

---

## Quick Start

### Option 1: Standalone REST + SPA Microservice (`app.rp`)

From the repository root:

```powershell
./raptor/bin/raptor.exe --go raptor/examples/todo/app.rp
```

By default, the server seeds sample tasks on first run and listens on **`http://localhost:8080`**.

Configure the port or SQLite database file with environment variables:

```powershell
$env:PORT = "9090"
$env:RAPTOR_TODO_DB = "my_tasks.db"
./raptor/bin/raptor.exe --go raptor/examples/todo/app.rp
```

### Option 2: RaptorHP Server-Rendered Template (`index.phtml`)

Run Raptor's built-in PHP-style development server (`-S`) pointed at `raptor/examples/todo`:

```powershell
./raptor/bin/raptor.exe -S localhost:8000 -t raptor/examples/todo
```

Then open **`http://localhost:8000`** in your browser. Both modes share `todo.db` (or `$RAPTOR_TODO_DB`).

---

## REST JSON API (`app.rp`)

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/` | Serves the responsive dark/light glassmorphism SPA |
| `GET` | `/api/todos` | Lists todos + summary counts (`?status=active\|completed`, `?priority=high\|medium\|low`, `?q=search`) |
| `POST` | `/api/todos` | Creates a new todo (`201 Created`) |
| `GET` | `/api/todos/:id` | Fetches a single todo by ID |
| `POST` | `/api/todos/:id/toggle` | Flips `completed` (`0` / `1`) for a todo |
| `PATCH` / `PUT` | `/api/todos/:id` | Updates `title`, `priority`, `category`, or `completed` |
| `DELETE` | `/api/todos/:id` | Deletes a single todo |
| `POST` | `/api/todos/clear-completed` | Deletes all completed todos |

### `curl` Examples

```bash
# 1. List all todos and summary counts
curl -s http://localhost:8080/api/todos

# 2. Filter active high-priority todos matching "api"
curl -s "http://localhost:8080/api/todos?status=active&priority=high&q=api"

# 3. Create a new todo (returns HTTP 201)
curl -s -X POST http://localhost:8080/api/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Deploy Raptor Todo service","priority":"high","category":"ops"}'

# 4. Toggle completion status of todo #1
curl -s -X POST http://localhost:8080/api/todos/1/toggle

# 5. Update title, priority, or category of todo #1
curl -s -X PATCH http://localhost:8080/api/todos/1 \
  -H "Content-Type: application/json" \
  -d '{"title":"Deploy Raptor Todo v1.1","priority":"medium","category":"release"}'

# 6. Delete todo #2
curl -s -X DELETE http://localhost:8080/api/todos/2

# 7. Clear all completed todos
curl -s -X POST http://localhost:8080/api/todos/clear-completed
```

---

## Running Automated Self-Tests & E2E Suite

Run the built-in ephemeral-port CRUD self-test in `app.rp`:

```powershell
./raptor/bin/raptor.exe --go raptor/examples/todo/app.rp --self-test
```

Run the full end-to-end TAP test suite exercising **both** `app.rp` and `index.phtml`:

```powershell
./raptor/bin/raptor.exe --go raptor/examples/todo/test_todo.rp
```
