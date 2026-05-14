# Atlas (Fork) - Manage Your Database Schema as Code

> **This is a community fork of [ariga/atlas](https://github.com/ariga/atlas).**
> Atlas Cloud API, version check, and other cloud-dependent features have been removed.
> This fork focuses on a standalone, self-contained CLI experience.

Atlas is a language-agnostic tool for managing and migrating database schemas using modern DevOps principles.
It offers two workflows:

- **Declarative**: Similar to Terraform, Atlas compares the current state of the database to the desired state, as
  defined in an [HCL], [SQL], or [ORM] schema. Based on this comparison, it generates and executes a migration plan to
  transition the database to its desired state.

- **Versioned**: Unlike other tools, Atlas automatically plans schema migrations for you. Users can describe their desired
  database schema in [HCL], [SQL], or their chosen [ORM], and by utilizing Atlas, they can plan, lint, and apply the
  necessary migrations to the database.

## Differences from Upstream

This fork removes the following from the official Atlas OSS build:

- **Atlas Cloud API** — GraphQL client, login/logout/whoami commands
- **Version check** — background phone-home notifications
- **Copilot integration** — AI copilot that depended on Atlas Cloud

## Supported Databases

[PostgreSQL](https://atlasgo.io/guides/postgres) ·
[MySQL](https://atlasgo.io/guides/mysql) ·
[MariaDB](https://atlasgo.io/guides/mysql) ·
[SQL Server](https://atlasgo.io/guides/mssql) ·
[SQLite](https://atlasgo.io/guides/sqlite) ·
[ClickHouse](https://atlasgo.io/guides/clickhouse) ·
[Redshift](https://atlasgo.io/guides/redshift) ·
[Oracle](https://atlasgo.io/guides/oracle) ·
[Snowflake](https://atlasgo.io/guides/snowflake) ·
[CockroachDB](https://atlasgo.io/guides/cockroachdb) ·
[TiDB](https://atlasgo.io/guides/mysql) ·
[Databricks](https://atlasgo.io/guides/databricks) ·
[Spanner](https://atlasgo.io/guides/spanner) ·
[Aurora DSQL](https://atlasgo.io/guides/dsql) ·
[Azure Fabric](https://atlasgo.io/guides/azure-fabric)

## Build from Source

```bash
git clone https://github.com/NFUChen/atlas.git
cd atlas/cmd/atlas
go build -o atlas .
```

## Key Features

- **[Declarative schema migrations](https://atlasgo.io/declarative/apply)**: The `atlas schema` command offers various options for [inspecting](https://atlasgo.io/inspect), diffing, comparing and applying migrations using standard Terraform-like workflows.
- **[Versioned migrations](https://atlasgo.io/versioned/intro)**: The `atlas migrate` command provides a state-of-the-art experience for [planning](https://atlasgo.io/versioned/diff), [linting](https://atlasgo.io/lint/analyzers), and [applying](https://atlasgo.io/versioned/apply) migrations.
- **[Schema as Code](https://atlasgo.io/atlas-schema)**: Define your desired database schema using [SQL], [HCL], or your chosen [ORM]. Atlas supports [16 ORM loaders](https://atlasgo.io/orms) across 6 languages.
- **[50+ safety analyzers](https://atlasgo.io/lint/analyzers)**: Database-aware migration linting that detects destructive changes, data-dependent modifications, table locks, backward-incompatible changes, and more.

## Getting Started

Inspect an existing database schema:
```shell
atlas schema inspect -u "postgres://localhost:5432/mydb"
```

Apply your desired schema to the database:
```shell
atlas schema apply \
  --url "postgres://localhost:5432/mydb" \
  --to file://schema.hcl \
  --dev-url "docker://postgres/16/dev"
```

## CLI Usage

### `schema inspect`

Inspect a specific MySQL schema and get its representation in Atlas DDL syntax:
```shell
atlas schema inspect -u "mysql://root:pass@localhost:3306/example" > schema.hcl
```

<details><summary>Result</summary>

```hcl
table "users" {
  schema = schema.example
  column "id" {
    null = false
    type = int
  }
  ...
}
```
</details>

Inspect the entire MySQL database and get its JSON representation:
```shell
atlas schema inspect \
  --url "mysql://root:pass@localhost:3306/" \
  --format '{{ json . }}' | jq
```

Inspect a PostgreSQL schema and get its ERD in Mermaid syntax:
```shell
atlas schema inspect \
  --url "postgres://root:pass@:5432/test?search_path=public&sslmode=disable" \
  --format '{{ mermaid . }}'
```

### `schema diff`

Compare two schema states and get a migration plan:
```shell
atlas schema diff \
  --from "postgres://postgres:pass@:5432/test?search_path=public&sslmode=disable" \
  --to file://schema.hcl \
  --dev-url "docker://postgres/15/test"
```

### `schema apply`

Generate a migration plan and apply it to the database:
```shell
atlas schema apply \
  --url mysql://root:pass@:3306/db1 \
  --to file://schema.hcl \
  --dev-url docker://mysql/8/db1
```

### `migrate diff`

Write a new migration file:
```shell
atlas migrate diff add_blog_posts \
  --dir file://migrations \
  --to file://schema.hcl \
  --dev-url docker://mysql/8/test
```

### `migrate apply`

Apply pending migration files:
```shell
atlas migrate apply \
  --url mysql://root:pass@:3306/db1 \
  --dir file://migrations
```

### `migrate lint`

Lint migration files for safety issues:
```bash
atlas migrate lint --dev-url "docker://postgres/16/dev"
```

## ORM Support

| Language | ORMs |
|----------|------|
| Go | [GORM](https://atlasgo.io/guides/orms/gorm), [Ent](https://atlasgo.io/guides/orms/ent), [Bun](https://atlasgo.io/guides/orms/bun), [Beego](https://atlasgo.io/guides/orms/beego), [sqlc](https://atlasgo.io/guides/frameworks/sqlc-versioned) |
| TypeScript | [Prisma](https://atlasgo.io/guides/orms/prisma), [Drizzle](https://atlasgo.io/guides/orms/drizzle), [TypeORM](https://atlasgo.io/guides/orms/typeorm), [Sequelize](https://atlasgo.io/guides/orms/sequelize) |
| Python | [Django](https://atlasgo.io/guides/orms/django), [SQLAlchemy](https://atlasgo.io/guides/orms/sqlalchemy) |
| Java | [Hibernate](https://atlasgo.io/guides/orms/hibernate) |
| .NET | [EF Core](https://atlasgo.io/guides/orms/efcore) |
| PHP | [Doctrine](https://atlasgo.io/guides/orms/doctrine) |

## Upstream

This project is forked from [ariga/atlas](https://github.com/ariga/atlas). For official documentation, visit [atlasgo.io](https://atlasgo.io).

[HCL]: https://atlasgo.io/atlas-schema/hcl
[SQL]: https://atlasgo.io/atlas-schema/sql
[ORM]: https://atlasgo.io/orms
