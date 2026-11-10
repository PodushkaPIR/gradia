# Gradia

Gradia is a local-first terminal app for tutors to organize students, track learning progress, and manage homework.

Your data stays on your computer in a SQLite database. No account, server, or cloud service is required.

## What it does

- Manage students and create individual learning plans from reusable templates.
- Organize topics, track progress, and keep teacher notes.
- Build a reusable task bank with images, PDF attachments, topics, difficulty levels, and collections.
- Compose homework from bank tasks, adjust their order, and track completion for each task.
- Share tasks through ZIP task packs and export learning reports as HTML or PDF.

The current interface is in Russian. PDF export is available for learning reports; PDFs containing homework tasks are not available yet.

## Run

Requires **Go 1.26 or newer** and an interactive terminal. From the project directory:

```sh
go run ./cmd/gradia
```

Use `--db` to choose a database file instead of the default user data location:

```sh
go run ./cmd/gradia --db ./gradia.db
```

Press `?` for help with the current screen.

Learning report PDF export requires a local **Chromium or Google Chrome** installation. Use `--pdf-browser /path/to/browser` if automatic detection cannot find it.
