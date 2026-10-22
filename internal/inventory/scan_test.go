package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func put(t *testing.T, root, path, content string) string {
	t.Helper()
	path = filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func encoded(t *testing.T, r *Report) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, Encode(&out, r))
	return out.String()
}

func TestScanDeclarativeFactsAndDeterminism(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "app"), filepath.Join(base, "platform")
	put(t, a, "compose.yaml", `services:
  web:
    image: nginx:1.27
    ports: ["127.0.0.1:8080:80", "53:53/udp", "${DYNAMIC_PORT}", "80-82"]
    expose: [80]
    depends_on: [db]
    environment:
      PASSWORD: DO_NOT_EMIT_THIS_VALUE
  db:
    image: postgres:17
    ports:
      - target: 5432
        published: "${HOST_PORT}"
        protocol: tcp
`)
	put(t, a, "package.json", `{
  "name": "frontend",
  "scripts": {"start": "DO_NOT_EMIT_THIS_VALUE"},
  "dependencies": {"react": "https://user:DO_NOT_EMIT_THIS_VALUE@example.invalid/pkg", "express": "^5"},
  "devDependencies": {"typescript": "^5"}
}`)
	put(t, a, "requirements.txt", "fastapi>=0.100\nrequests @ https://user:DO_NOT_EMIT_THIS_VALUE@example.invalid/requests.whl\n-r private.txt\n")
	put(t, b, "main.tf", `resource "aws_vpc" "main" {}`)
	put(t, b, "pyproject.toml", "[project]\nname = 'backend'\n")
	put(t, b, "resources.yaml", `apiVersion: apps/v1
kind: Deployment
metadata: {name: api}
spec:
  selector: {matchLabels: {app: api}}
  template:
    metadata: {labels: {app: api}}
    spec:
      containers: [{name: api, image: nginx:1.27, ports: [{containerPort: 8080}]}]
---
apiVersion: v1
kind: Service
metadata: {name: api}
spec:
  selector: {app: api}
  ports: [{name: http, port: 80, targetPort: 8080}]
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: {name: entry}
spec: {gatewayClassName: envoy, listeners: [{name: http, port: 80, protocol: HTTP}]}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: {name: api}
spec:
  parentRefs: [{name: entry}]
  rules: [{backendRefs: [{name: api, port: 80}]}]
---
apiVersion: v1
kind: Secret
metadata: {name: password}
stringData: {password: DO_NOT_EMIT_THIS_VALUE}
`)
	r, err := Scan(context.Background(), []string{b, a}, "test")
	require.NoError(t, err)
	out := encoded(t, r)
	assert.NotContains(t, out, "DO_NOT_EMIT_THIS_VALUE")
	assert.NotContains(t, out, "example.invalid")
	require.Len(t, r.Packages, 2)
	assert.Equal(t, "frontend", r.Packages[0].Name)
	assert.Equal(t, []string{"express", "react", "typescript"}, depNames(r.Packages[0]))
	assert.Equal(t, []string{"fastapi", "requests"}, depNames(r.Packages[1]))
	var webFound, dbFound, routeFound, dependencyFound bool
	for _, e := range r.Elements {
		require.NotEmpty(t, e.Src.File)
		require.Positive(t, e.Src.Line)
		if e.Title == "web" {
			webFound = true
			assert.Equal(t, "Docker Compose · nginx:1.27", e.Tech)
			require.Len(t, e.Ports, 2)
			assert.Equal(t, 53, e.Ports[0].Number)
			assert.Equal(t, "UDP", e.Ports[0].Protocol)
			assert.Equal(t, 80, e.Ports[1].Number)
			assert.Equal(t, "TCP", e.Ports[1].Protocol)
		}
		if e.Title == "db" {
			dbFound = true
			require.Len(t, e.Ports, 1)
			assert.Equal(t, 5432, e.Ports[0].Number)
		}
	}
	for _, c := range r.Connections {
		assert.True(t, strings.HasPrefix(c.ID, "k8s:"), "Compose dependencies are not traffic")
		if strings.Contains(c.From, "/gateway/entry") && strings.Contains(c.To, "/service/api") {
			routeFound = true
		}
	}
	for _, ref := range r.References {
		if ref.Kind == "depends_on" {
			dependencyFound = true
			assert.Contains(t, ref.To, "/service/db")
		}
	}
	assert.True(t, webFound && dbFound && routeFound && dependencyFound)
	assert.Contains(t, out, "port-not-literal")
	assert.Contains(t, out, "parser-not-implemented")
	for _, f := range r.Files {
		assert.Len(t, f.SHA256, 64)
	}
	// Output stored inside a source tree must not become an input on the next scan.
	put(t, a, "inventory.yaml", out)
	t.Setenv("DYNAMIC_PORT", "3000")
	again, err := Scan(context.Background(), []string{a, b, a}, "test")
	require.NoError(t, err)
	assert.Equal(t, out, encoded(t, again))
	validateSchema(t, out)
}

func depNames(p Package) []string {
	var out []string
	for _, dep := range p.Dependencies {
		out = append(out, dep.Name)
	}
	return out
}

func TestScanDoesNotFollowSymlinksOrReadCredentialAndGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{".env", ".env.yaml", "credentials.json", "terraform.tfstate", "terraform.tfvars.json", "node_modules/package.json", ".git/compose.yaml", "dist/compose.yaml", ".aws/credentials"} {
		put(t, root, path, "DO_NOT_READ: [\n")
	}
	outside := put(t, t.TempDir(), "compose.yaml", "services: {outside: {image: nginx:1.27}}\n")
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "compose.yaml")))
	r, err := Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	assert.Empty(t, r.Files)
	assert.Empty(t, r.Elements)
}

func TestScanReportsRenderingAndAmbiguousDocuments(t *testing.T) {
	root := t.TempDir()
	put(t, root, "chart/Chart.yaml", "apiVersion: v2\nname: app\nversion: 1.0.0\n")
	put(t, root, "chart/templates/service.yaml", "{{ DO_NOT_EMIT_THIS_VALUE }}\n")
	put(t, root, "overlay/kustomization.yaml", "resources: [deployment.yaml]\n")
	put(t, root, "overlay/deployment.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata: {name: raw}\n")
	put(t, root, "bad/compose.yaml", "services:\n  api: {}\n  api: {}\n")
	put(t, root, "bad/package.json", "{\"name\": }\n")
	put(t, root, "bad/resources.yaml", "apiVersion: v1\nkind: Service\nmetadata: {name: api}\n---\nbroken: [\n")
	put(t, root, "large/compose.yaml", strings.Repeat(" ", maxFileBytes+1))
	r, err := Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	assert.Empty(t, r.Elements)
	assert.NotContains(t, encoded(t, r), "DO_NOT_EMIT_THIS_VALUE")
	statuses := map[string]string{}
	for _, f := range r.Files {
		statuses[f.Path] = f.Status
	}
	assert.Equal(t, "render-required", statuses["chart/templates/service.yaml"])
	assert.Equal(t, "render-required", statuses["overlay/deployment.yaml"])
	assert.Equal(t, "unresolved", statuses["bad/compose.yaml"])
	assert.Equal(t, "unresolved", statuses["bad/package.json"])
	assert.Equal(t, "unresolved", statuses["bad/resources.yaml"])
	assert.Equal(t, "unresolved", statuses["large/compose.yaml"])
	validateSchema(t, encoded(t, r))
}

func TestComposeAliasesMissingDependenciesAndNoTraffic(t *testing.T) {
	root := t.TempDir()
	put(t, root, "compose.yaml", `x-common: &common
  image: nginx:1.27
  expose: [8080]
services:
  api:
    <<: *common
    depends_on: {missing: {condition: service_started}}
  worker: *common
`)
	r, err := Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	require.Len(t, r.Elements, 2)
	for _, e := range r.Elements {
		require.Len(t, e.Ports, 1)
		assert.Equal(t, 8080, e.Ports[0].Number)
	}
	assert.Empty(t, r.Connections)
	assert.Empty(t, r.References)
	assert.Contains(t, encoded(t, r), "missing-or-dynamic-dependency")
}

func TestSameNamedRootsHaveDistinctStableIDsAndCancellation(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a", "app"), filepath.Join(base, "b", "app")
	for _, root := range []string{a, b} {
		put(t, root, "compose.yaml", "services: {api: {image: nginx:1.27}}\n")
	}
	r, err := Scan(context.Background(), []string{a, b}, "")
	require.NoError(t, err)
	require.Len(t, r.Elements, 2)
	assert.NotEqual(t, r.Elements[0].ID, r.Elements[1].ID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Scan(ctx, []string{a}, "")
	assert.ErrorIs(t, err, context.Canceled)
	_, err = Scan(context.Background(), []string{filepath.Join(base, "missing")}, "")
	assert.Error(t, err)
}

func TestKubernetesDirectoriesDoNotInventCrossEnvironmentConnections(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"dev", "prod"} {
		put(t, root, dir+"/service.yaml", "apiVersion: v1\nkind: Service\nmetadata: {name: api}\nspec: {ports: [{name: http, port: 80}]}\n")
	}
	put(t, root, "edge/gateway.yaml", `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata: {name: entry}
spec: {gatewayClassName: envoy, listeners: [{name: http, port: 80, protocol: HTTP}]}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata: {name: api}
spec: {parentRefs: [{name: entry}], rules: [{backendRefs: [{name: api, port: 80}]}]}
`)
	r, err := Scan(context.Background(), []string{root}, "default")
	require.NoError(t, err)
	var services []string
	for _, e := range r.Elements {
		if strings.Contains(e.ID, "/service/api") {
			services = append(services, e.ID)
		}
	}
	require.Len(t, services, 2)
	assert.NotEqual(t, services[0], services[1])
	assert.Empty(t, r.Connections, "a service in another directory is not implicitly in this deployment")
	assert.Contains(t, encoded(t, r), "missing-backend")
}

func TestComposeRejectsAmbiguityWithoutDanglingReferences(t *testing.T) {
	root := t.TempDir()
	put(t, root, "compose.yaml", `services:
  api:
    image: ${IMAGE}
    expose: ["8080:80"]
    depends_on: [broken]
  broken: null
`)
	r, err := Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	require.Len(t, r.Elements, 1)
	assert.Empty(t, r.Elements[0].Ports)
	assert.Empty(t, r.References)
	assert.NotContains(t, encoded(t, r), "${IMAGE}")
	put(t, root, "compose.yaml", "services: {api: {}}\n---\nservices: {db: {}}\n")
	r, err = Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	assert.Empty(t, r.Elements)
	assert.Equal(t, "unresolved", r.Files[0].Status)
}

func TestKubernetesDynamicDeclarationsAndCyclicAliasesAreUnresolved(t *testing.T) {
	root := t.TempDir()
	put(t, root, "dynamic.yaml", "apiVersion: v1\nkind: Service\nmetadata: {name: api}\nspec: {ports: [{port: '${PORT}'}]}\n")
	put(t, root, "cycle.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: cycle}\ndata: &cycle {self: *cycle}\n")
	put(t, root, "image.yaml", "apiVersion: v1\nkind: Pod\nmetadata: {name: bad-image}\nspec: {containers: [{name: api, image: 'https://user:DO_NOT_EMIT_THIS_VALUE@registry.invalid/image'}]}\n")
	r, err := Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	assert.Empty(t, r.Elements)
	assert.NotContains(t, encoded(t, r), "${PORT}")
	assert.NotContains(t, encoded(t, r), "DO_NOT_EMIT_THIS_VALUE")
	assert.Contains(t, encoded(t, r), "dynamic-declaration")
	assert.Contains(t, encoded(t, r), "invalid-document")
}

func TestInvalidKubernetesPortsRemainUnknown(t *testing.T) {
	root := t.TempDir()
	put(t, root, "service.yaml", "apiVersion: v1\nkind: Service\nmetadata: {name: api}\nspec: {ports: [{port: 0}, {port: 70000}, {port: 80, protocol: HTTP}]}\n")
	r, err := Scan(context.Background(), []string{root}, "")
	require.NoError(t, err)
	for _, e := range r.Elements {
		assert.Empty(t, e.Ports)
	}
	assert.Contains(t, encoded(t, r), "invalid-port-declaration")
	validateSchema(t, encoded(t, r))
}

func validateSchema(t *testing.T, content string) {
	t.Helper()
	c := jsonschema.NewCompiler()
	for _, file := range []string{"../../schemas/v1alpha1/fragment.schema.json", "../../schemas/inventory/v1alpha1/inventory.schema.json"} {
		path, err := filepath.Abs(file)
		require.NoError(t, err)
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		require.NoError(t, err)
		require.NoError(t, c.AddResource(path, doc))
		require.NoError(t, c.AddResource(doc.(map[string]any)["$id"].(string), doc))
	}
	schema, err := c.Compile("https://github.com/TheGostsniperfr/Noodle/schemas/inventory/v1alpha1/inventory.schema.json")
	require.NoError(t, err)
	var value any
	require.NoError(t, yaml.Unmarshal([]byte(content), &value))
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	require.NoError(t, schema.Validate(doc))
}
