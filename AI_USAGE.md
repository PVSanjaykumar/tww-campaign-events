# AI Usage Disclosure

- **Tool used:** ChatGPT.
- **What I used it for:** Understanding the assignment, discussing design trade-offs, drafting the initial implementation of the Go HTTP handlers and in-memory store, creating focused tests, investigating the four debugging defects, and preparing documentation.
- **One suggestion I changed:** The assignment allows SQLite or in-memory storage. I chose in-memory storage rather than adding a SQLite driver because the time box is short and the standard-library implementation keeps the focus on idempotency, validation, concurrency, and API behaviour. The trade-off is that data is lost when the process restarts.
- **One thing AI helped me understand:** A mutex must protect the complete read-modify-write operation on shared counters; otherwise concurrent increments can be lost even when each individual increment looks simple.
- **AI-generated work I reviewed and tested:** The initial handlers, store, and tests were AI-assisted. I ran `gofmt`, `go test ./...`, and `go test -race ./...`. I also checked the debugging exercise against its exact expected output and ran its race detector. I will review and be prepared to explain each part before submission.
