# Traefik Request Body Rewrite Plugin

A [Traefik](https://traefik.io/) middleware plugin to perform regex-based rewrites on HTTP request bodies, with optional filtering by HTTP method, request path, and content type.

## Features

* **Regex Replacements:** Define any number of find-and-replace rules using Go regexes.
* **Per-Rule Filters:** Each rewrite rule can be scoped to:

  * **HTTP Methods** (e.g. `POST`, `PUT`)
  * **Content Types** (e.g. `application/json`)
  * **URL Path Patterns** (via regex against `req.URL.Path`)
* **Safe Streaming:** Reads full body, applies rewriting, and updates the `Content-Length` header.
* **Zero Dependencies:** Pure Go implementation—no external SDK needed.

## Installation

1. **Clone the repo**

   ```bash
   git clone https://github.com/maretodoric/traefik-plugin-requestbodyrewrite.git
   cd traefik-plugin-requestbodyrewrite
   ```

2. **Configure Traefik**

   ```yaml
   # traefik.yml (static config)
   experimental:
     plugins:
       requestbodyrewrite:
         moduleName: "github.com/maretodoric/traefik-plugin-requestbodyrewrite"
         version: "v1.0.0"

   http:
     routers:
       my-router:
         rule: "Host(`example.com`)"
         service: my-service
         middlewares:
           - rewrite-req
     middlewares:
       rewrite-req:
         plugin:
           requestbodyrewrite:
             rewrites:
               - regex: "foo"
                 replacement: "bar"
                 methods:
                    - POST
                 contentTypes:
                    - "application/json"
                 pathRegex: "/api/.*$"
   ```

3. **Restart Traefik**

## Configuration

```yaml
# config.yml (dynamic)
http:
  middlewares:
    rewrite-req:
      plugin:
        requestbodyrewrite:
          rewrites:
            # Rule #1: POST JSON to /api/users
            - regex: "user_id=\d+"
              replacement: "user_id=anonymous"
              methods: ["POST"]
              contentTypes: ["application/json"]
              pathRegex: "^/api/users$"

            # Rule #2: All form submissions
            - regex: "password=secret"
              replacement: "password=********"
              contentTypes: ["application/x-www-form-urlencoded"]
```

## License

MIT © Marko Todorić
