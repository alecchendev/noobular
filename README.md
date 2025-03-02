# Noobular

Noobular is a software tutor made to help people learn as effectively as possible.
[Math Academy](mathacademy.com) has proven that it's possible to build a software
system that dramatically increases the rate of learning, using various techniques
supported by decades of evidence. I'm personally a very satisfied customer of
Math Academy, and I want to expand the reach of Math Academy's learnings to teach
and learn subjects beyond math. TL;DR, Noobular is a Math Academy duplicate where
anyone can upload content.

### Development

Dependencies:
- Go
- Sqlite

Run server locally:
- `go run cmd/server/main.go`

Run tests:
- `go test ./...`

Checks:
- Linting: `golangci-lint run`
- Formatting: `gofumpt -l -w .`

### Noobular rewrite

In February 2025, I'm rewriting Noobular.

Outline
- Make things extremely simple, prioritize features perfectly fit for myself
because I am the only user right now.
    - No web UI to create content. Only a UI preview, enroll, and take courses.
    - CLI for creating content. Save content as text files locally,
    then upload them via the CLI.
    - No versioning for content at first. No editing, only creation and deletion.
- Tech stack
    - Main logic: Golang, use fmt/linting
    - DB: Sqlite, no ORM, yes migration tool
    - Frontend: HTMX
    - Hosting/deployment: raspberry pi, cloudflare tunnels
- Main features
    - Create knowledge points - set of questions
    - Create modules - sequence of content and knowledge points
    - Create course - set of modules with prerequisite relationships
    - Enroll in course
    - Content engine - render content, pick knowledge points, track answers
    - Diagnostic exam
    - Take modules - introduce new content, interactive, finely scaffolded
    - Review - spaced repetition to solidify long-term memory, address weak spots
    - Quizzes - measure learning

Current experiment
- I need a way to prove to myself whether Noobular actually works.
- Two stretch goals:
    - Use Noobular to teach someone else something: Hand-make a beginner
    cryptography course. I can personally evaluate the effectiveness,
    and can ask someone in my life to volunteer as a student.
    - Use Noobular to teach myself something: Use AI to generate content
    to teach me chemistry or physics. Try to pass the AP exam in ~2 months
    (some arbitrary fraction of a semester). This is not very advanced material,
    so AI should be accurate/reliable enough for this, though I will
    need to experiment a lot to get this to work well.

To do
- Build the basic system
- Conduct experiments
    - Make cryptography course
    - Chemsitry/physics
