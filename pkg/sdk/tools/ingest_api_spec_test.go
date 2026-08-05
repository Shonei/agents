package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOpenAPI3(t *testing.T) {
	raw := []byte(`
openapi: 3.0.3
info:
  title: Shop API
  version: 1.0.0
  description: Demo shop
servers:
  - url: https://api.example.com/v1
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
paths:
  /orders:
    get:
      operationId: listOrders
      tags: [Orders]
      summary: List orders
      security:
        - bearerAuth: []
      parameters:
        - name: status
          in: query
          schema:
            type: string
      responses:
        "200":
          description: OK
    post:
      tags: [Orders]
      summary: Create order
      responses:
        "201":
          description: Created
  /products/{id}:
    get:
      tags: [Products]
      summary: Get product
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
`)

	result, err := parseAPISpec(raw, "test.yaml", "", "", 250)
	require.NoError(t, err)
	assert.Equal(t, specFormatOpenAPI3, result.Format)
	assert.Equal(t, "Shop API", result.Title)
	assert.Equal(t, []string{"https://api.example.com/v1"}, result.Servers)
	require.Len(t, result.SecuritySchemes, 1)
	assert.Equal(t, "bearerAuth", result.SecuritySchemes[0].Name)
	assert.Equal(t, 3, result.OperationCount)
	require.Len(t, result.Operations, 3)

	list := result.Operations[0]
	assert.Equal(t, "GET", list.Method)
	assert.Equal(t, "/orders", list.Path)
	assert.Equal(t, "listOrders", list.ID)
	assert.Equal(t, "orders", list.SuggestedGroup)
	assert.Equal(t, "list_orders.md", list.SuggestedFile)
	assert.Contains(t, result.SuggestedGroups, "orders")
	assert.Contains(t, result.SuggestedGroups, "products")
}

func TestParseOpenAPI3_Filters(t *testing.T) {
	raw := []byte(`{
	  "openapi": "3.0.0",
	  "info": {"title": "T", "version": "1"},
	  "paths": {
	    "/orders": {
	      "get": {"tags": ["Orders"], "summary": "list"}
	    },
	    "/products": {
	      "get": {"tags": ["Products"], "summary": "list"}
	    }
	  }
	}`)

	byPrefix, err := parseAPISpec(raw, "x.json", "/orders", "", 250)
	require.NoError(t, err)
	assert.Equal(t, 1, byPrefix.OperationCount)

	byTag, err := parseAPISpec(raw, "x.json", "", "products", 250)
	require.NoError(t, err)
	assert.Equal(t, 1, byTag.OperationCount)
	assert.Equal(t, "/products", byTag.Operations[0].Path)
}

func TestParseOpenAPI3_Truncate(t *testing.T) {
	raw := []byte(`{
	  "openapi": "3.0.0",
	  "info": {"title": "T", "version": "1"},
	  "paths": {
	    "/a": {"get": {"summary": "a"}},
	    "/b": {"get": {"summary": "b"}},
	    "/c": {"get": {"summary": "c"}}
	  }
	}`)

	result, err := parseAPISpec(raw, "x.json", "", "", 2)
	require.NoError(t, err)
	assert.Equal(t, 3, result.OperationCount)
	assert.Equal(t, 1, result.TruncatedOperations)
	require.Len(t, result.Operations, 2)
}

func TestParsePostman(t *testing.T) {
	raw := []byte(`{
	  "info": {
	    "name": "Demo",
	    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	  },
	  "item": [
	    {
	      "name": "Orders",
	      "item": [
	        {
	          "name": "List Orders",
	          "request": {
	            "method": "GET",
	            "url": "{{baseUrl}}/orders"
	          }
	        }
	      ]
	    }
	  ]
	}`)

	result, err := parseAPISpec(raw, "collection.json", "", "", 250)
	require.NoError(t, err)
	assert.Equal(t, specFormatPostman, result.Format)
	assert.Equal(t, "Demo", result.Title)
	require.Len(t, result.Operations, 1)
	assert.Equal(t, "GET", result.Operations[0].Method)
	assert.Equal(t, "{{baseUrl}}/orders", result.Operations[0].Path)
	assert.Equal(t, "orders", result.Operations[0].SuggestedGroup)
}

func TestIngestAPISpecTool_FromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openapi.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
	  "openapi": "3.0.0",
	  "info": {"title": "Local", "version": "1"},
	  "paths": {"/ping": {"get": {"summary": "ping"}}}
	}`), 0o600))

	tool := &IngestAPISpecTool{}
	out, err := tool.Call(map[string]interface{}{"path": path})
	require.NoError(t, err)
	result, ok := out.(*SpecIngestResult)
	require.True(t, ok)
	assert.Equal(t, "Local", result.Title)
	assert.Equal(t, 1, result.OperationCount)
}

func TestIngestAPISpecTool_RequiresURLOrPath(t *testing.T) {
	tool := &IngestAPISpecTool{}
	_, err := tool.Call(map[string]interface{}{})
	require.Error(t, err)
}
