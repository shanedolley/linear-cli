#!/bin/bash
set -e

# Read API key from auth file
AUTH_FILE="$HOME/.lincli-auth.json"
if [ ! -f "$AUTH_FILE" ]; then
    echo "Error: Auth file not found at $AUTH_FILE"
    echo "Run 'lincli auth' to authenticate first"
    exit 1
fi

API_KEY=$(jq -r '.api_key' "$AUTH_FILE")
if [ -z "$API_KEY" ] || [ "$API_KEY" = "null" ]; then
    echo "Error: No API key found in $AUTH_FILE"
    exit 1
fi

echo "Downloading Linear GraphQL schema..."

# Create directory if it doesn't exist
mkdir -p pkg/api

# Get introspection query from genqlient
INTROSPECTION_QUERY='query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type {
    ...TypeRef
  }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
              }
            }
          }
        }
      }
    }
  }
}'

# Download schema and convert to SDL format
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $API_KEY" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg query "$INTROSPECTION_QUERY" '{query: $query}')" \
  | jq -r '.data' > /tmp/linear-schema-introspection.json

# Check if introspection was successful
if [ ! -s /tmp/linear-schema-introspection.json ] || [ "$(cat /tmp/linear-schema-introspection.json)" = "null" ]; then
    echo "Error: Failed to download schema from Linear API"
    rm -f /tmp/linear-schema-introspection.json
    exit 1
fi

# Convert JSON introspection result to SDL using npx graphql-json-to-sdl
if ! command -v npx &> /dev/null; then
    echo "Error: npx (Node.js) is required but not installed"
    echo "Please install Node.js from https://nodejs.org/"
    rm -f /tmp/linear-schema-introspection.json
    exit 1
fi

npx -y graphql-json-to-sdl /tmp/linear-schema-introspection.json pkg/api/schema.graphql

# Clean up
rm -f /tmp/linear-schema-introspection.json

echo "✓ Schema saved to pkg/api/schema.graphql"

# Show some stats
TYPES_COUNT=$(grep -c "^type " pkg/api/schema.graphql || echo "0")
INPUTS_COUNT=$(grep -c "^input " pkg/api/schema.graphql || echo "0")
ENUMS_COUNT=$(grep -c "^enum " pkg/api/schema.graphql || echo "0")
FILE_SIZE=$(wc -c < pkg/api/schema.graphql | tr -d ' ')

echo ""
echo "Schema statistics:"
echo "  File size: ${FILE_SIZE} bytes"
echo "  Types: ${TYPES_COUNT}"
echo "  Inputs: ${INPUTS_COUNT}"
echo "  Enums: ${ENUMS_COUNT}"
