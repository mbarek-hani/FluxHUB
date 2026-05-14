# FluxHUB Refactoring Plan

This document serves as a step-by-step plan for a full refactor and code cleaning of the FluxHUB application. It is intended to guide LLMs and developers through the necessary changes systematically.

## 1. Code Reorganization & Clean Up
- **Minimize `main.go`**: Extract all routing logic from `main.go` into a dedicated `routes` package to keep the entry point clean and maintainable.
- **Remove AI Artifacts**: Scour the codebase to remove any unpolished or unprofessional artifacts, specifically eliminating all emojis from log messages and comments.
- **Structured Logging**: Replace all standard `log.Print` or `fmt.Println` statements with a structured logging library (e.g., Go 1.21's `slog`) everywhere that matters. Logs should be professional, consistent, and easily searchable.
- **Code Organization**: Better organize all packages and code logic. Ensure a strict separation of concerns among controllers, services, models, and routes.

## 2. Frontend & Templating
- **Migrate to `templ`**: Completely replace the standard `html/template` package with `templ`. This will provide type-safe, compiled HTML templating and easier component building.
- **Migrate to Tailwind CSS**: Replace all existing vanilla CSS with Tailwind CSS to speed up development and standardize the design system.
- **Reusable UI Components**: Break down frequently used interfaces into highly organized, reusable `templ` components (e.g., `Button`, `Input`, `Alert`, `Card`).
- **Clean UI & Dark Mode**: Develop a clean, modern user interface that fully supports both Dark Mode and Light Mode natively (utilizing Tailwind's dark mode capabilities).
- **Branding**: The application name is **FluxHUB**. Ensure its branding is prominent and utilize its logo (`static/icons/favicon.svg`) in the interface (e.g., in the navbar, login page, and sidebar).

## 3. Users, Roles & Authentication
- **Unified Database Table**: Ensure all user types (Admins and Developers) are stored in the exact same database table (e.g., `users`), distinguished by a `role` column.
- **Admin Creation (CLI Only)**: Utilize or refactor the existing command-line tooling (`cmd/admin/main.go`) to securely create Admin accounts. The web interface must not have the capability to create Admins.
- **Developer Creation (UI)**: Developers will be created/registered via the web UI registration form.
- **Unified Login System**: Implement a single login interface (`/login`). Upon successful authentication, the system must read the user's role and route them to their respective dashboard (Admin Dashboard vs Developer Dashboard).
- **Database-Backed Sessions**: Implement a secure session management system. 
  - Sessions must be stored in a dedicated database table.
  - Sessions and their corresponding cookies must have an expiration time of exactly **1 day**.

## 4. Database Migrations
- **GORM & Manual Migrations**: We are using GORM for database operations. It is crucial to strictly remove any automatic migrations (e.g., `db.AutoMigrate()`) from the main application startup sequence.
- **Implement/Refactor Migration Commands**: Utilize, refactor, or enhance the existing system for running database migrations (`cmd/migrate/main.go`) to safely and manually handle database schema changes.

## 5. Documentation & Developer Experience
- **Update `README.md`**: Create a comprehensive README file that includes:
  - High-level information about the FluxHUB project.
  - Instructions on how to install `air` (the Go live-reloading tool).
  - Instructions on how to run the project using `air` for development.
  - Explicit instructions on how to manually run database migrations using the provided command (`cmd/migrate/main.go`).

---

## Step-by-Step Execution Plan

### Phase 1: Setup and Documentation
1. Create and populate the new `README.md` with project details, `air` installation/run instructions, and manual database migration instructions.
2. Initialize Tailwind CSS in the project and configure `tailwind.config.js`.
3. Set up the `templ` generation pipeline (e.g., integrating `templ generate` with `air`).

### Phase 2: Core Refactoring
1. Implement the structured logging mechanism (e.g., `slog`) and scrub all emojis from the codebase.
2. Create the `routes` package, migrate all handlers out of `main.go`, and establish clear folder structures.
3. Clean up the rest of the existing codebase to fit the new organization.

### Phase 3: Database & Auth Infrastructure
1. Refactor or enhance the existing database migration tooling (`cmd/migrate/main.go`). Ensure any `AutoMigrate` calls are removed from the main application flow.
2. Write migrations for the `users` table (with role support) and the `sessions` table.
3. Refactor or enhance the existing CLI command for creating Admin users (`cmd/admin/main.go`).

### Phase 4: UI & Templating Conversion
1. Build the base layout in `templ` supporting Light/Dark modes and including the FluxHUB logo.
2. Create the core reusable UI components (`Button`, `Input`, `Alert`, etc.) in `templ` using Tailwind CSS.
3. Convert all existing HTML templates to `templ` components.
4. Remove the old `html/template` implementations and vanilla CSS files.

### Phase 5: Authentication Logic
1. Implement the database-backed session logic (creation, validation, and 1-day expiration).
2. Build the unified Login `templ` page.
3. Implement the login handler with role-based redirection to the appropriate dashboards.
4. Implement the developer web UI registration handler.
