# Gist List API

HTTP API that returns a GitHub user's public gists. Built to the Operability take-home requirements: list gists for `GET /<USER>`, automated tests, and a container listening on port `8080`.

## Requirements

- Go 1.22 or later (to run locally)
- Docker (to run the container)

No third-party Go modules, private registries, or organisation-specific credentials are used. The API talks only to the public GitHub gist API (no token required). Docker images are pulled from Docker Hub.

## Layout

```
cmd/api/            application entrypoint
internal/github/    GitHub gist client
internal/server/    HTTP handlers
```

## Run tests

```bash
go test ./...
```

Tests use `octocat` as the example user and mock the GitHub API, so they do not need network access.

## Run locally

```bash
go run ./cmd/api
```

The server listens on `8080` by default.

```bash
curl http://localhost:8080/octocat
```

Example response:

```json
{
  "username": "octocat",
  "count": 1,
  "gists": [
    {
      "id": "aa5a315d61ae9438b18d",
      "description": "Hello World Examples",
      "html_url": "https://gist.github.com/aa5a315d61ae9438b18d",
      "public": true,
      "files": ["hello_world.rb"],
      "created_at": "2010-04-14T02:15:15Z",
      "updated_at": "2011-06-20T11:34:15Z"
    }
  ]
}
```

`GET /healthz` returns `{"status":"ok"}` and does not call GitHub.

## Run with Docker

```bash
docker build -t gist-api .
docker run --rm -p 8080:8080 gist-api
```

Then:

```bash
curl http://localhost:8080/octocat
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | Listen port |
| `GITHUB_TOKEN` | unset | Optional GitHub token. Public gists work without it; a token raises the rate limit. |
| `GITHUB_API_URL` | `https://api.github.com` | Override GitHub API base URL (used in tests) |

## Behaviour

- `GET /<USER>` calls [`GET /users/{username}/gists`](https://docs.github.com/en/rest/gists/gists?apiVersion=2022-11-28) and returns public gists for that user.
- Pagination is followed (up to 10 pages of 100 gists) so the response is a full list, not only the first page.
- Unknown users return `404`. Invalid usernames return `400`. GitHub failures return `502`.
