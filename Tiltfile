# localmap Tiltfile
# Supports two modes:
#   - Local (default): runs `go run` directly, no container needed
#   - Container: builds and runs the image from Dockerfile via podman-compose
#     (set LOCALMAP_MODE=container)

mode = os.environ.get('LOCALMAP_MODE', 'local')

# ---------------------------------------------------------------------------
# Environment
# Load .env if present; fall back to .env.example values as defaults.
# ---------------------------------------------------------------------------
env_file = '.env' if os.path.exists('.env') else '.env.example'

env_vars = {
  'LM_MAP_LAT_DD': '47.62155335517305',
  'LM_MAP_LON_DD': '-122.3495095751273',
  'LM_MAP_ZOOM': '12',
  'LM_AISCATCHER_URL': 'http://192.168.1.25:8100',
  'LM_CACHEDMAP_QUERY': 'profile=trmnl_v1_alias',
  'LM_CACHEDMAP_TTL': '30s',
}

addr_port = int(env_vars.get('LM_ADDR', ':8080').lstrip(':'))

readiness_probe_for = lambda: probe(
    http_get=http_get_action(
        port=addr_port,
        path='/',
    ),
    period_secs=3,
    failure_threshold=10,
)

links_for = lambda: [link('http://localhost:%d' % addr_port, 'localmap')]

if mode == 'container':
    # -------------------------------------------------------------------
    # Container mode: build the image from Dockerfile and run it with
    # podman-compose (docker-compose.yml), driven via podman instead of
    # docker since this host has no `docker`/`docker-compose` binaries.
    # -------------------------------------------------------------------
    def _sh_quote(value):
        return "'" + value.replace("'", "'\\''") + "'"

    def _env_prefix(vars):
        return ' '.join(['%s=%s' % (k, _sh_quote(v)) for k, v in vars.items()])

    compose_env = _env_prefix(env_vars)

    local_resource(
        'localmap',
        cmd='%s podman-compose build' % compose_env,
        serve_cmd='%s podman-compose up' % compose_env,
        deps=['cmd', 'internal', 'go.mod', 'go.sum', 'Dockerfile', 'docker-compose.yml'],
        labels=['server'],
        links=links_for(),
        readiness_probe=readiness_probe_for(),
    )
else:
    # -------------------------------------------------------------------
    # Local mode: run the Go binary directly (no Docker / Kubernetes needed)
    # -------------------------------------------------------------------
    local_env_vars = dict(env_vars)
    local_env_vars.setdefault('CHROME_PATH', '/usr/bin/google-chrome')

    local_resource(
        'localmap',
        serve_cmd='go run ./cmd/localmap',
        serve_env=local_env_vars,
        deps=['cmd', 'internal', 'go.mod', 'go.sum'],
        labels=['server'],
        links=links_for(),
        readiness_probe=readiness_probe_for(),
    )
