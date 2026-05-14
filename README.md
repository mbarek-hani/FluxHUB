# FluxHUB

FluxHUB is a modern web application built using Go, Templ, Tailwind CSS, and GORM.

## Development Setup

### Prerequisites

You need the following installed to develop and run FluxHUB:
- [Go](https://go.dev/doc/install)
- [Node.js & npm](https://nodejs.org/en/download/) (for Tailwind CSS)
- [Air](https://github.com/cosmtrek/air) (for live-reloading)
- [Templ](https://templ.guide/quick-start/installation) (for compiling `.templ` files)

### Installing Air & Templ

To install `air` for Go live-reloading:
```bash
go install github.com/cosmtrek/air@latest
```

To install `templ`:
```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### Initializing the Project

First, set up your environment variables and generate an application encryption key:
```bash
cp .example.env .env
go run cmd/key/main.go
```

Then, install the necessary Node.js dependencies for Tailwind CSS:
```bash
npm install
```

### Running the Application

To run the application with live-reloading enabled, simply run:
```bash
air
```

This command will watch for changes in your `.go`, `.templ`, and `.css` files. It will automatically run `templ generate`, compile the Tailwind CSS, and restart the Go application server.

## Database Migrations

FluxHUB uses GORM for database interactions but **strictly requires manual database migrations**. We do not use automatic migrations (`AutoMigrate`) in the main application flow to ensure schema safety and predictability.

To run database migrations, execute the following command:
```bash
go run cmd/migrate/main.go
```
This script will safely apply any required schema changes to the database.

## Admin Creation

Admin accounts cannot be created via the web interface. To create an admin account, use the CLI tool:
```bash
go run cmd/admin/main.go
```
