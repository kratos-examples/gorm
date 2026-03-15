[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-examples/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-examples/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-examples)](https://pkg.go.dev/github.com/yylego/kratos-examples)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-examples/main.svg)](https://coveralls.io/github/yylego/kratos-examples?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.25+-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-examples.svg)](https://github.com/yylego/kratos-examples/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-examples)](https://goreportcard.com/report/github.com/yylego/kratos-examples)

# kratos-examples

Demo projects built with the Go-Kratos framework.

---

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->

## CHINESE README

[中文说明](README.zh.md)

<!-- TEMPLATE (EN) END: LANGUAGE NAVIGATION -->

## Introduction

**kratos-examples** is a reference implementation demonstrating best practices when building microservices with the [Go-Kratos](https://go-kratos.dev) framework. It serves as:

- 🎯 **Foundation Project** - The upstream template of 20+ specialized demo projects in the kratos-orz ecosystem
- 🛠️ **Toolchain Integration Example** - Showcasing kratos-orz development tools in action
- 📚 **Learning Resource** - Complete microservice structure following Kratos conventions
- ⚡ **Fast Development** - Auto-sync proto and code through magic commands like make orz

## About Go-Kratos

[Go-Kratos](https://go-kratos.dev) is a concise and efficient microservice framework that provides:

- Clean architecture with distinct separation of concerns
- Comprehensive middleware and plugin ecosystem
- Built-in gRPC and HTTP protocol support
- Excellent documentation and active ecosystem

**kratos-examples builds upon this solid foundation**, adding enhanced tooling and automation to streamline the development workflow.

## Core Features

### 🚀 kratos-orz Toolchain Integration

Provides kratos-orz toolchain:

- **orzkratos-add-proto** - Simplifies adding proto files to the Kratos project
- **orzkratos-srv-proto** - Auto syncs service implementations with proto definitions

Setup tools:

```bash
make init
```

### ⚡ Magic Command: `make orz`

The core feature - auto synchronization between proto files and service code:

```bash
make orz
```

**What it does:**

- ✅ New methods in proto → Auto generates service method stubs
- ✅ Deleted methods → Converts to unexported functions (preserves logic)
- ✅ Reordered methods → Auto rearranges service code to match proto sequence

**Workflow example:**

1. Add `CreateArticle` method to the proto file
2. Run `make orz`
3. Service generates `CreateArticle` method stub
4. Implement the business logic

### 🔀 Fork Project Synchronization

Provides complete automated workflow to sync fork projects with upstream changes.

Through `make merge-stepN` series commands, auto handles upstream code merging, conflict resolution, dependencies upgrades, test validation, and more.

See [Makefile](./Makefile) with detailed workflow and usage instructions.

## Project Structure

### Demo Projects

Provides two demos to showcase the usage of various features:

- [demo1kratos](./demo1kratos) - Student CRUD microservice (simple Kratos example)
- [demo2kratos](./demo2kratos) - Article CRUD microservice (advanced features and integrations)

Both demos follow standard Kratos project structure with proto-first design, Wire DI, and gRPC/HTTP endpoints.

### Parameter Validation

The project demonstrates a two-tier validation pattern:

- **Service Layer** - Returns `ErrorBadParam` (HTTP 400) for invalid input, giving clients actionable feedback
- **Biz Layer** - Uses `must` assertions as a safety net, catching programming mistakes that bypass service validation

### Data Layer

The source project uses `gofakeit` to generate mock data, keeping the focus on framework structure and toolchain integration. Fork projects (such as [gorm](https://github.com/kratos-examples/gorm)) replace mock data with real database operations using GORM + SQLite, demonstrating production-grade CRUD patterns with `gormrepo`.

We provide a code comparison between Demo1 (base) and Demo2 (fork), highlighting the changed code blocks.

When this project is forked, you can also compare it with the source to see the differences.

### Code Changes

The [changes/](./changes) section contains markdown files documenting code differences:

- [changes/demo1.md](./changes/demo1.md) - Demo1 changes compared to source
- [changes/demo2.md](./changes/demo2.md) - Demo2 changes compared to source
- [changes/aside.md](changes/aside.md) - Aside modules and sibling projects

Tests auto-generate these files:

```bash
go test -v -run TestGenerateDemo1Changes # Generate demo1.md
go test -v -run TestGenerateDemo2Changes # Generate demo2.md
go test -v -run TestGenerateAsideChanges # Generate aside.md
```

**In source project:** Files show `✅ NO CHANGES`

**In fork projects:** Files show code differences with syntax highlighting, making it simple to track customizations on GitHub.

## Forks

|    demo     |                      repo                      |
| :---------: | :--------------------------------------------: |
|     ast     |     https://github.com/kratos-examples/ast     |
| custom-auth | https://github.com/kratos-examples/custom-auth |
| static-auth | https://github.com/kratos-examples/static-auth |
|   config    |   https://github.com/kratos-examples/config    |
|    cron     |    https://github.com/kratos-examples/cron     |
|     ebz     |     https://github.com/kratos-examples/ebz     |
|    cobra    |    https://github.com/kratos-examples/cobra    |
|    gorm     |    https://github.com/kratos-examples/gorm     |
|    cors     |    https://github.com/kratos-examples/cors     |
|    i18n     |    https://github.com/kratos-examples/i18n     |
|    nacos    |    https://github.com/kratos-examples/nacos    |
|   swaggo    |   https://github.com/kratos-examples/swaggo    |
|    trace    |    https://github.com/kratos-examples/trace    |
|    test     |    https://github.com/kratos-examples/test     |
|    vue3     |    https://github.com/kratos-examples/vue3     |
|    wire     |    https://github.com/kratos-examples/wire     |
|     zap     |     https://github.com/kratos-examples/zap     |
|    zapzh    |    https://github.com/kratos-examples/zapzh    |
|   migrate   |   https://github.com/kratos-examples/migrate   |
|    ping     |    https://github.com/kratos-examples/ping     |
| supervisord | https://github.com/kratos-examples/supervisord |

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-11-25 03:52:28.131064 +0000 UTC -->

## 📄 License

MIT License - see [LICENSE](LICENSE).

---

## 💬 Contact & Feedback

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Mistake reports?** Open an issue on GitHub with reproduction steps
- 💡 **Fresh ideas?** Create an issue to discuss
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a merge request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉🎉🎉

<!-- TEMPLATE (EN) END: STANDARD PROJECT FOOTER -->

---

## GitHub Stars

[![Stargazers](https://starchart.cc/yylego/kratos-examples.svg?variant=adaptive)](https://starchart.cc/yylego/kratos-examples)
